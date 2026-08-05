package workspace

import (
	"context"
	"errors"
	"net/netip"
	"strconv"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (s *workspacesService) SetAuthPolicy(
	ctx context.Context,
	workspaceID uuid.UUID,
	enforcement entity.AuthEnforcement,
) (service.AuthPolicyOutcome, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAuthPolicy,
		Action:      entity.ActionUpdate,
		WorkspaceID: workspaceID,
	}); err != nil {
		return service.AuthPolicyOutcome{}, err
	}

	if !enforcement.Valid() {
		return service.AuthPolicyOutcome{}, entity.NewValidationError(entity.FieldError{
			Field: "enforcement",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	var outcome service.AuthPolicyOutcome

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.workspaces.LockByIDs(ctx, []uuid.UUID{workspaceID}); err != nil {
			return err
		}

		if enforcement == entity.AuthEnforcementSSO {
			if err := s.readyForEnforcement(ctx, workspaceID); err != nil {
				return err
			}

			codes, err := s.issueRecoveryCodes(ctx, workspaceID)
			if err != nil {
				return err
			}

			outcome.RecoveryCodes = codes
		} else if err := s.breakGlass.Discard(ctx, workspaceID); err != nil {
			return err
		}

		policy, err := s.authPolicies.Upsert(ctx, entity.WorkspaceAuthPolicy{
			WorkspaceID: workspaceID,
			Enforcement: enforcement,
		})
		if err != nil {
			return err
		}

		outcome.Policy = policy

		return nil
	})
	if err != nil {
		return service.AuthPolicyOutcome{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditSSOEnforcement,
		ResourceKind: string(entity.ResourceAuthPolicy),
		ResourceID:   workspaceID,
		Detail:       map[string]string{"enforcement": string(enforcement)},
	})

	if len(outcome.RecoveryCodes) > 0 {
		s.audit.Record(ctx, entity.AuditEntry{
			WorkspaceID:  workspaceID,
			Action:       entity.AuditRecoveryCodesIssued,
			ResourceKind: string(entity.ResourceAuthPolicy),
			ResourceID:   workspaceID,
			Detail:       map[string]string{"codes": strconv.Itoa(len(outcome.RecoveryCodes))},
		})
	}

	return outcome, nil
}

func (s *workspacesService) readyForEnforcement(
	ctx context.Context,
	workspaceID uuid.UUID,
) error {
	verified, err := s.connections.Verified(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, entity.ErrSSOConnectionNotFound) {
			return entity.EnforcementRefusedError{Blocker: entity.EnforcementBlockerNoConnection}
		}

		return err
	}

	if !verified {
		return entity.EnforcementRefusedError{Blocker: entity.EnforcementBlockerNotVerified}
	}

	linked, err := s.identities.AnyLinkedAdmin(ctx, workspaceID)
	if err != nil {
		return err
	}

	if !linked {
		return entity.EnforcementRefusedError{Blocker: entity.EnforcementBlockerNoLinkedAdmin}
	}

	return nil
}

func (s *workspacesService) issueRecoveryCodes(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]string, error) {
	codes := make([]string, 0, entity.BreakGlassCodeCount)
	hashes := make([][]byte, 0, entity.BreakGlassCodeCount)

	for range entity.BreakGlassCodeCount {
		code, hash, err := entity.NewBreakGlassCode()
		if err != nil {
			return nil, err
		}

		codes = append(codes, code)
		hashes = append(hashes, hash)
	}

	var issuer *uuid.UUID
	if actor, ok := identity.Actor(ctx); ok {
		accountID := actor.AccountID
		issuer = &accountID
	}

	if err := s.breakGlass.Replace(ctx, workspaceID, issuer, hashes); err != nil {
		return nil, err
	}

	return codes, nil
}

func (s *workspacesService) EnforcementReadiness(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.EnforcementBlocker, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAuthPolicy,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return "", err
	}

	err := s.readyForEnforcement(ctx, workspaceID)
	if err == nil {
		return "", nil
	}

	var refused entity.EnforcementRefusedError
	if errors.As(err, &refused) {
		return refused.Blocker, nil
	}

	return "", err
}

func (s *workspacesService) RedeemRecoveryCode(
	ctx context.Context,
	input service.RedeemRecoveryCodeInput,
) error {
	workspace, err := s.workspaces.GetBySlug(ctx, input.WorkspaceSlug)
	if err != nil {
		if errors.Is(err, entity.ErrWorkspaceNotFound) {
			return entity.ErrBreakGlassCodeInvalid
		}

		return err
	}

	policy, err := s.authPolicies.Get(ctx, workspace.ID)
	if err != nil {
		return err
	}

	if policy.Enforcement != entity.AuthEnforcementSSO {
		return entity.ErrBreakGlassNotEnforcing
	}

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.breakGlass.Redeem(
			ctx,
			workspace.ID,
			entity.HashBreakGlassCode(input.Code),
			input.From,
		); err != nil {
			return err
		}

		_, err := s.authPolicies.Upsert(ctx, entity.WorkspaceAuthPolicy{
			WorkspaceID: workspace.ID,
			Enforcement: entity.AuthEnforcementAny,
		})

		return err
	})
	if err != nil {
		return err
	}

	logging.From(ctx).WarnContext(
		ctx,
		"single sign-on requirement lifted with a recovery code",
		"workspace_id", workspace.ID.String(),
		"workspace_slug", workspace.Slug,
		"from", input.From,
	)

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:   workspace.ID,
		WorkspaceName: workspace.Name,
		Action:        entity.AuditRecoveryCodeRedeemed,
		Actor:         entity.AuditActor{Kind: entity.ActorKindSystem},
		SourceIP:      redeemedFrom(input.From),
		ResourceKind:  string(entity.ResourceAuthPolicy),
		ResourceID:    workspace.ID,
	})

	return nil
}

func (s *workspacesService) ListSSOIdentities(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.SSOIdentity, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceMembership,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return nil, err
	}

	return s.identities.ListByWorkspaceID(ctx, workspaceID)
}

func (s *workspacesService) UnlinkSSOIdentity(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
) error {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceMembership,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	}); err != nil {
		return err
	}

	if err := s.identities.Unlink(ctx, workspaceID, accountID); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditSSOIdentityUnlinked,
		ResourceKind: string(entity.ResourceAccount),
		ResourceID:   accountID,
	})

	return nil
}

func redeemedFrom(raw string) netip.Addr {
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}
	}

	return addr
}

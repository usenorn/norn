package apitoken

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const usageStampInterval = time.Minute

type apiTokensService struct {
	tokens     repository.APIToken
	accounts   repository.Account
	agents     repository.Agent
	workspaces repository.Workspace
	mailer     repository.Mailer
	authorizer service.Authorizer
	transactor repository.Transactor
	app        config.App
	audit      service.Audit
}

func New(
	tokens repository.APIToken,
	accounts repository.Account,
	agents repository.Agent,
	workspaces repository.Workspace,
	mailer repository.Mailer,
	authorizer service.Authorizer,
	transactor repository.Transactor,
	app config.App,
	audit service.Audit,
) service.APITokens {
	return &apiTokensService{
		tokens:     tokens,
		accounts:   accounts,
		agents:     agents,
		workspaces: workspaces,
		mailer:     mailer,
		authorizer: authorizer,
		transactor: transactor,
		app:        app,
		audit:      audit,
	}
}

func (s *apiTokensService) Mint(
	ctx context.Context,
	input service.MintAPITokenInput,
) (service.MintedAPIToken, error) {
	minter, err := s.self(ctx)
	if err != nil {
		return service.MintedAPIToken{}, err
	}

	if err := entity.NewValidationError(entity.ValidateAPITokenName("name", input.Name)); err != nil {
		return service.MintedAPIToken{}, err
	}

	scopes := input.Scopes.Normalized()

	if len(scopes) == 0 || len(scopes) != len(input.Scopes) {
		return service.MintedAPIToken{}, entity.ErrAPITokenScopeInvalid
	}

	if len(input.Grants) == 0 {
		return service.MintedAPIToken{}, entity.ErrAPITokenGrantMissing
	}

	if err := s.checkGrants(ctx, scopes, input.Grants); err != nil {
		return service.MintedAPIToken{}, err
	}

	expiresAt := input.ExpiresAt

	if expiresAt == nil || expiresAt.After(time.Now().UTC().Add(entity.APITokenMaxTTL)) {
		deadline := time.Now().UTC().Add(entity.APITokenMaxTTL)
		expiresAt = &deadline
	}

	value, tokenHash, err := entity.NewAPIToken()
	if err != nil {
		return service.MintedAPIToken{}, err
	}

	var token entity.APIToken

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		created, err := s.tokens.Create(ctx, entity.APIToken{
			AccountID: minter.AccountID,
			Name:      input.Name,
			TokenHash: tokenHash,
			Scopes:    scopes,
			Grants:    input.Grants,
			ExpiresAt: expiresAt,
		})
		if err != nil {
			return err
		}

		token = created

		return nil
	}); err != nil {
		return service.MintedAPIToken{}, err
	}

	for _, grant := range input.Grants {
		s.audit.Record(ctx, entity.AuditEntry{
			WorkspaceID:  grant.WorkspaceID,
			Action:       entity.AuditTokenMinted,
			ResourceKind: string(entity.ResourceAPIToken),
			ResourceID:   token.ID,
			ResourceName: token.Name,
		})
	}

	return service.MintedAPIToken{Token: token, Value: value}, nil
}

func (s *apiTokensService) checkGrants(
	ctx context.Context,
	scopes entity.APIScopeSet,
	grants entity.APITokenGrants,
) error {
	seen := make(map[uuid.UUID]struct{}, len(grants))

	for _, grant := range grants {
		if _, duplicate := seen[grant.WorkspaceID]; duplicate {
			return entity.ErrAPITokenGrantInvalid
		}

		seen[grant.WorkspaceID] = struct{}{}

		decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
			Resource:    entity.ResourceIssue,
			Action:      entity.ActionRead,
			WorkspaceID: grant.WorkspaceID,
			Scoped:      true,
		})
		if err != nil {
			return entity.ErrAPITokenGrantInvalid
		}

		if !scopes.SubsetOf(entity.AllowedAPIScopesFor(decision.Role)) {
			return entity.ErrAPITokenScopeExceeds
		}

		if grant.AllTeams && len(grant.TeamIDs) > 0 {
			return entity.ErrAPITokenGrantInvalid
		}

		if !grant.AllTeams && len(grant.TeamIDs) == 0 {
			return entity.ErrAPITokenGrantInvalid
		}

		for _, teamID := range grant.TeamIDs {
			if !decision.Scope.Covers(teamID) {
				return entity.ErrAPITokenGrantInvalid
			}
		}
	}

	return nil
}

func (s *apiTokensService) List(ctx context.Context) ([]entity.APIToken, error) {
	owner, err := s.self(ctx)
	if err != nil {
		return nil, err
	}

	return s.tokens.ListByOwner(ctx, owner.AccountID)
}

func (s *apiTokensService) ListForWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]service.OwnedAPIToken, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAPIToken,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return nil, err
	}

	tokens, err := s.tokens.ListByWorkspaceGrant(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	owned := make([]service.OwnedAPIToken, 0, len(tokens))

	for _, token := range tokens {
		account, err := s.accounts.GetByID(ctx, token.AccountID)
		if err != nil {
			return nil, err
		}

		owned = append(owned, service.OwnedAPIToken{
			Token:      token,
			OwnerName:  account.DisplayName,
			OwnerEmail: account.Email,
		})
	}

	return owned, nil
}

func (s *apiTokensService) Revoke(ctx context.Context, tokenID uuid.UUID) error {
	owner, err := s.self(ctx)
	if err != nil {
		return err
	}

	token, err := s.tokens.GetByID(ctx, tokenID)
	if err != nil {
		return err
	}

	if token.AccountID != owner.AccountID {
		return entity.ErrAPITokenNotFound
	}

	if err := s.tokens.Revoke(ctx, tokenID, time.Now().UTC()); err != nil {
		return err
	}

	s.recordRevocation(ctx, token)

	return nil
}

func (s *apiTokensService) RevokeInWorkspace(ctx context.Context, workspaceID, tokenID uuid.UUID) error {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAPIToken,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.ErrAPITokenMintForbidden
	}

	token, err := s.tokens.GetByID(ctx, tokenID)
	if err != nil {
		return err
	}

	if !token.Grants.Covers(workspaceID) {
		return entity.ErrAPITokenNotFound
	}

	if err := s.tokens.Revoke(ctx, tokenID, time.Now().UTC()); err != nil {
		return err
	}

	s.recordRevocation(ctx, token)

	return nil
}

func (s *apiTokensService) Authenticate(ctx context.Context, value string) (entity.Actor, error) {
	token, err := s.tokens.GetByTokenHash(ctx, entity.HashAPIToken(value))
	if err != nil {
		return entity.Actor{}, err
	}

	now := time.Now().UTC()

	if !token.Usable(now) {
		return entity.Actor{}, entity.ErrAPITokenNotFound
	}

	account, err := s.accounts.GetByID(ctx, token.AccountID)
	if err != nil {
		return entity.Actor{}, err
	}

	if account.Status != entity.AccountStatusActive {
		return entity.Actor{}, entity.ErrAPITokenNotFound
	}

	if token.NeedsUsageStamp(now, usageStampInterval) {
		if err := s.tokens.RecordUsage(ctx, token.ID, now); err != nil {
			logging.From(ctx).WarnContext(
				ctx,
				"recording api token usage failed",
				"token_id", token.ID.String(),
				"error", err.Error(),
			)
		}
	}

	tokenID := token.ID

	actor := entity.Actor{
		Kind:      entity.ActorKindToken,
		AccountID: token.AccountID,
		TokenID:   &tokenID,
		TokenName: token.Name,
		Grants:    token.Grants,
		Scopes:    token.Scopes,
	}

	if !account.Agent() {
		return actor, nil
	}

	actor.Kind = entity.ActorKindAgent

	agent, err := s.agents.GetByAccountID(ctx, token.AccountID)
	if err != nil {
		return entity.Actor{}, err
	}

	if agent.Disabled() {
		return entity.Actor{}, entity.ErrAgentDisabled
	}

	agentID := agent.ID

	actor.AgentID = &agentID
	actor.AgentAllowance = agent.Allowance()
	actor.OwnerAccountID = agent.OwnerAccountID

	return actor, nil
}

func (s *apiTokensService) self(ctx context.Context) (entity.Actor, error) {
	actor, ok := identity.Actor(ctx)
	if !ok {
		return entity.Actor{}, entity.ErrAccountForbidden
	}

	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource: entity.ResourceAPIToken,
		Action:   entity.ActionManage,
		Subject:  actor.AccountID,
	})
	if err != nil {
		return entity.Actor{}, err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.Actor{}, entity.ErrAPITokenMintForbidden
	}

	return decision.Actor, nil
}

func (s *apiTokensService) recordRevocation(ctx context.Context, token entity.APIToken) {
	for _, grant := range token.Grants {
		s.audit.Record(ctx, entity.AuditEntry{
			WorkspaceID:  grant.WorkspaceID,
			Action:       entity.AuditTokenRevoked,
			ResourceKind: string(entity.ResourceAPIToken),
			ResourceID:   token.ID,
			ResourceName: token.Name,
		})
	}
}

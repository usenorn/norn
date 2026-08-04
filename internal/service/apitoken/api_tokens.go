package apitoken

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const usageStampInterval = time.Minute

type apiTokensService struct {
	tokens      repository.APIToken
	memberships repository.Membership
	accounts    repository.Account
	authorizer  service.Authorizer
}

func New(
	tokens repository.APIToken,
	memberships repository.Membership,
	accounts repository.Account,
	authorizer service.Authorizer,
) service.APITokens {
	return &apiTokensService{
		tokens:      tokens,
		memberships: memberships,
		accounts:    accounts,
		authorizer:  authorizer,
	}
}

func (s *apiTokensService) Mint(
	ctx context.Context,
	input service.MintAPITokenInput,
) (service.MintedAPIToken, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAPIToken,
		Action:      entity.ActionManage,
		WorkspaceID: input.WorkspaceID,
	})
	if err != nil {
		return service.MintedAPIToken{}, err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return service.MintedAPIToken{}, entity.ErrAPITokenMintForbidden
	}

	if err := entity.NewValidationError(entity.ValidateAPITokenName("name", input.Name)); err != nil {
		return service.MintedAPIToken{}, err
	}

	scopes := input.Scopes.Normalized()

	if len(scopes) == 0 || len(scopes) != len(input.Scopes) {
		return service.MintedAPIToken{}, entity.ErrAPITokenScopeInvalid
	}

	if !scopes.SubsetOf(entity.AllowedAPIScopesFor(decision.Role)) {
		return service.MintedAPIToken{}, entity.ErrAPITokenScopeExceeds
	}

	expiresAt := input.ExpiresAt

	if expiresAt == nil {
		deadline := time.Now().UTC().Add(entity.APITokenMaxTTL)
		expiresAt = &deadline
	}

	value, tokenHash, err := entity.NewAPIToken()
	if err != nil {
		return service.MintedAPIToken{}, err
	}

	token, err := s.tokens.Create(ctx, entity.APIToken{
		AccountID:   decision.Actor.AccountID,
		WorkspaceID: input.WorkspaceID,
		Name:        input.Name,
		TokenHash:   tokenHash,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return service.MintedAPIToken{}, err
	}

	return service.MintedAPIToken{Token: token, Value: value}, nil
}

func (s *apiTokensService) List(ctx context.Context, workspaceID uuid.UUID) ([]entity.APIToken, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAPIToken,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}

	return s.tokens.ListByOwner(ctx, workspaceID, decision.Actor.AccountID)
}

func (s *apiTokensService) Revoke(ctx context.Context, workspaceID, tokenID uuid.UUID) error {
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

	return s.tokens.Revoke(ctx, workspaceID, decision.Actor.AccountID, tokenID, time.Now().UTC())
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

	membership, err := s.memberships.Get(ctx, token.WorkspaceID, token.AccountID)
	if err != nil {
		return entity.Actor{}, err
	}

	account, err := s.accounts.GetByID(ctx, token.AccountID)
	if err != nil {
		return entity.Actor{}, err
	}

	if account.Status != entity.AccountStatusActive {
		return entity.Actor{}, entity.ErrAPITokenNotFound
	}

	scopes := make(entity.APIScopeSet, 0, len(token.Scopes))
	allowed := entity.AllowedAPIScopesFor(membership.Role)

	for _, scope := range token.Scopes {
		if allowed.Permits(scope.Resource(), scope.Action()) {
			scopes = append(scopes, scope)
		}
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
	workspaceID := token.WorkspaceID

	kind := entity.ActorKindToken
	if account.Agent() {
		kind = entity.ActorKindAgent
	}

	return entity.Actor{
		Kind:        kind,
		AccountID:   token.AccountID,
		TokenID:     &tokenID,
		WorkspaceID: &workspaceID,
		Scopes:      scopes,
	}, nil
}

package apitoken_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	apitokenrepo "github.com/usenorn/norn/internal/repository/apitoken"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	"github.com/usenorn/norn/internal/service"
	apitokensvc "github.com/usenorn/norn/internal/service/apitoken"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
)

type harness struct {
	tokens      *apitokenrepo.MockAPIToken
	memberships *membershiprepo.MockMembership
	accounts    *accountrepo.MockAccount
	authorizer  *authorizersvc.MockAuthorizer
	service     service.APITokens
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		tokens:      apitokenrepo.NewMockAPIToken(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
	}

	h.service = apitokensvc.New(h.tokens, h.memberships, h.accounts, h.authorizer)

	return h
}

func (h *harness) actingAs(kind entity.ActorKind, role entity.MembershipRole) uuid.UUID {
	accountID := uuid.New()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: kind, AccountID: accountID},
			Role:  role,
		}, nil)

	return accountID
}

func readScopes() entity.APIScopeSet {
	return entity.APIScopeSet{entity.NewAPIScope(entity.ResourceTeam, entity.ActionRead)}
}

func manageScopes() entity.APIScopeSet {
	return entity.APIScopeSet{entity.NewAPIScope(entity.ResourceTeam, entity.ActionManage)}
}

func TestAMintedTokenReturnsItsSecretOnceAndStoresOnlyTheHash(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.actingAs(entity.ActorKindUser, entity.MembershipRoleAdmin)

	var stored entity.APIToken

	h.tokens.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token entity.APIToken) (entity.APIToken, error) {
			stored = token

			return token, nil
		})

	minted, err := h.service.Mint(context.Background(), service.MintAPITokenInput{
		WorkspaceID: workspaceID,
		Name:        "CI",
		Scopes:      readScopes(),
	})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	if !strings.HasPrefix(minted.Value, entity.APITokenPrefix) {
		t.Fatalf("token %q does not carry the %q prefix that makes a leak greppable", minted.Value, entity.APITokenPrefix)
	}

	if strings.Contains(string(stored.TokenHash), minted.Value) {
		t.Fatal("the stored hash contains the plaintext token")
	}

	if string(stored.TokenHash) != string(entity.HashAPIToken(minted.Value)) {
		t.Fatal("the stored hash is not the hash of the returned token")
	}
}

func TestATokenMayNotExceedItsCreatorsRights(t *testing.T) {
	h := newHarness(t)

	h.actingAs(entity.ActorKindUser, entity.MembershipRoleMember)

	_, err := h.service.Mint(context.Background(), service.MintAPITokenInput{
		WorkspaceID: uuid.New(),
		Name:        "CI",
		Scopes:      manageScopes(),
	})

	if !errors.Is(err, entity.ErrAPITokenScopeExceeds) {
		t.Fatalf("Mint error = %v, want ErrAPITokenScopeExceeds", err)
	}
}

func TestATokenMayNotMintAnotherToken(t *testing.T) {
	h := newHarness(t)

	h.actingAs(entity.ActorKindToken, entity.MembershipRoleAdmin)

	_, err := h.service.Mint(context.Background(), service.MintAPITokenInput{
		WorkspaceID: uuid.New(),
		Name:        "CI",
		Scopes:      readScopes(),
	})

	if !errors.Is(err, entity.ErrAPITokenMintForbidden) {
		t.Fatalf("Mint error = %v, want ErrAPITokenMintForbidden", err)
	}
}

func TestATokenMayNotRevokeAnotherToken(t *testing.T) {
	h := newHarness(t)

	h.actingAs(entity.ActorKindToken, entity.MembershipRoleAdmin)

	err := h.service.Revoke(context.Background(), uuid.New(), uuid.New())

	if !errors.Is(err, entity.ErrAPITokenMintForbidden) {
		t.Fatalf("Revoke error = %v, want ErrAPITokenMintForbidden", err)
	}
}

func TestAnUnknownScopeIsRefused(t *testing.T) {
	h := newHarness(t)

	h.actingAs(entity.ActorKindUser, entity.MembershipRoleAdmin)

	_, err := h.service.Mint(context.Background(), service.MintAPITokenInput{
		WorkspaceID: uuid.New(),
		Name:        "CI",
		Scopes:      entity.APIScopeSet{"team:destroy"},
	})

	if !errors.Is(err, entity.ErrAPITokenScopeInvalid) {
		t.Fatalf("Mint error = %v, want ErrAPITokenScopeInvalid", err)
	}
}

func TestATokenWithNoScopesIsRefused(t *testing.T) {
	h := newHarness(t)

	h.actingAs(entity.ActorKindUser, entity.MembershipRoleAdmin)

	_, err := h.service.Mint(context.Background(), service.MintAPITokenInput{
		WorkspaceID: uuid.New(),
		Name:        "CI",
	})

	if !errors.Is(err, entity.ErrAPITokenScopeInvalid) {
		t.Fatalf("Mint error = %v, want ErrAPITokenScopeInvalid", err)
	}
}

func storedToken(workspaceID, accountID uuid.UUID, scopes entity.APIScopeSet) entity.APIToken {
	return entity.APIToken{
		ID:          uuid.New(),
		AccountID:   accountID,
		WorkspaceID: workspaceID,
		Name:        "CI",
		TokenHash:   entity.HashAPIToken("nrn_value"),
		Scopes:      scopes,
	}
}

func TestAuthenticatingATokenConfinesItToItsWorkspaceAndScopes(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	accountID := uuid.New()
	token := storedToken(workspaceID, accountID, readScopes())

	h.tokens.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(token, nil)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, accountID).
		Return(entity.Membership{Role: entity.MembershipRoleAdmin}, nil)
	h.accounts.EXPECT().
		GetByID(gomock.Any(), accountID).
		Return(entity.Account{ID: accountID, Status: entity.AccountStatusActive}, nil)
	h.tokens.EXPECT().RecordUsage(gomock.Any(), token.ID, gomock.Any()).Return(nil)

	actor, err := h.service.Authenticate(context.Background(), "nrn_value")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if actor.Kind != entity.ActorKindToken {
		t.Fatalf("actor kind = %q, want token", actor.Kind)
	}

	if actor.WorkspaceID == nil || *actor.WorkspaceID != workspaceID {
		t.Fatal("a token actor must be confined to the workspace it was minted for")
	}

	if actor.Holds(entity.NewPermission(entity.ResourceTeam, entity.ActionManage)) {
		t.Fatal("the actor holds a permission its token was never granted")
	}
}

func TestADemotedCreatorNarrowsTheTokenImmediately(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	accountID := uuid.New()
	token := storedToken(workspaceID, accountID, manageScopes())

	h.tokens.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(token, nil)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, accountID).
		Return(entity.Membership{Role: entity.MembershipRoleMember}, nil)
	h.accounts.EXPECT().
		GetByID(gomock.Any(), accountID).
		Return(entity.Account{ID: accountID, Status: entity.AccountStatusActive}, nil)
	h.tokens.EXPECT().RecordUsage(gomock.Any(), token.ID, gomock.Any()).Return(nil)

	actor, err := h.service.Authenticate(context.Background(), "nrn_value")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if actor.Holds(entity.NewPermission(entity.ResourceTeam, entity.ActionManage)) {
		t.Fatal("a demoted creator must narrow the token at once, not at the next mint")
	}
}

func TestARevokedOrExpiredTokenAuthenticatesAsNobody(t *testing.T) {
	revoked := time.Now().UTC().Add(-time.Hour)
	expired := time.Now().UTC().Add(-time.Minute)

	cases := map[string]func(entity.APIToken) entity.APIToken{
		"revoked": func(token entity.APIToken) entity.APIToken {
			token.RevokedAt = &revoked

			return token
		},
		"expired": func(token entity.APIToken) entity.APIToken {
			token.ExpiresAt = &expired

			return token
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			token := mutate(storedToken(uuid.New(), uuid.New(), readScopes()))

			h.tokens.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(token, nil)

			_, err := h.service.Authenticate(context.Background(), "nrn_value")
			if !errors.Is(err, entity.ErrAPITokenNotFound) {
				t.Fatalf("Authenticate error = %v, want ErrAPITokenNotFound", err)
			}
		})
	}
}

func TestATokenBelongingToADeactivatedAccountAuthenticatesAsNobody(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	accountID := uuid.New()
	token := storedToken(workspaceID, accountID, readScopes())

	h.tokens.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(token, nil)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, accountID).
		Return(entity.Membership{Role: entity.MembershipRoleAdmin}, nil)
	h.accounts.EXPECT().
		GetByID(gomock.Any(), accountID).
		Return(entity.Account{ID: accountID, Status: entity.AccountStatusDeactivated}, nil)

	_, err := h.service.Authenticate(context.Background(), "nrn_value")
	if !errors.Is(err, entity.ErrAPITokenNotFound) {
		t.Fatalf("Authenticate error = %v, want ErrAPITokenNotFound", err)
	}
}

func TestUsageIsStampedAtMostOncePerInterval(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	accountID := uuid.New()
	token := storedToken(workspaceID, accountID, readScopes())
	justNow := time.Now().UTC()
	token.LastUsedAt = &justNow

	h.tokens.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(token, nil)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, accountID).
		Return(entity.Membership{Role: entity.MembershipRoleAdmin}, nil)
	h.accounts.EXPECT().
		GetByID(gomock.Any(), accountID).
		Return(entity.Account{ID: accountID, Status: entity.AccountStatusActive}, nil)

	if _, err := h.service.Authenticate(context.Background(), "nrn_value"); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
}

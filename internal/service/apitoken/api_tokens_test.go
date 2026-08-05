package apitoken_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	apitokenrepo "github.com/usenorn/norn/internal/repository/apitoken"
	mailerrepo "github.com/usenorn/norn/internal/repository/mailer"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	apitokensvc "github.com/usenorn/norn/internal/service/apitoken"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
)

type access struct {
	role     entity.MembershipRole
	allTeams bool
	teams    []uuid.UUID
}

type harness struct {
	tokens     *apitokenrepo.MockAPIToken
	accounts   *accountrepo.MockAccount
	workspaces *workspacerepo.MockWorkspace
	mailer     *mailerrepo.MockMailer
	authorizer *authorizersvc.MockAuthorizer
	service    service.APITokens

	accountID uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		tokens:     apitokenrepo.NewMockAPIToken(ctrl),
		accounts:   accountrepo.NewMockAccount(ctrl),
		workspaces: workspacerepo.NewMockWorkspace(ctrl),
		mailer:     mailerrepo.NewMockMailer(ctrl),
		authorizer: authorizersvc.NewMockAuthorizer(ctrl),
		accountID:  uuid.New(),
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = apitokensvc.New(
		h.tokens,
		h.accounts,
		agentrepo.NewMockAgent(ctrl),
		h.workspaces,
		h.mailer,
		h.authorizer,
		transactor,
		config.App{BaseURL: "https://norn.test"},
		silentAudit(ctrl),
	)

	return h
}

func (h *harness) actingAs(kind entity.ActorKind, workspaces map[uuid.UUID]access) context.Context {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			actor := entity.Actor{Kind: kind, AccountID: h.accountID}

			if request.WorkspaceID == uuid.Nil {
				return entity.Decision{Actor: actor}, nil
			}

			granted, ok := workspaces[request.WorkspaceID]
			if !ok {
				return entity.Decision{}, entity.ErrAccountForbidden
			}

			return entity.Decision{
				Actor: actor,
				Role:  granted.role,
				Scope: entity.TeamScope{
					WorkspaceID: request.WorkspaceID,
					AllTeams:    granted.allTeams,
					TeamIDs:     granted.teams,
				},
			}, nil
		}).
		AnyTimes()

	return identity.Into(context.Background(), h.accountID)
}

func readScopes() entity.APIScopeSet {
	return entity.APIScopeSet{entity.NewAPIScope(entity.ResourceTeam, entity.ActionRead)}
}

func manageScopes() entity.APIScopeSet {
	return entity.APIScopeSet{entity.NewAPIScope(entity.ResourceTeam, entity.ActionManage)}
}

func wholeWorkspace(workspaceID uuid.UUID) entity.APITokenGrants {
	return entity.APITokenGrants{{WorkspaceID: workspaceID, AllTeams: true}}
}

func TestAMintedTokenReturnsItsSecretOnceAndStoresOnlyTheHash(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		workspaceID: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	var stored entity.APIToken

	h.tokens.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token entity.APIToken) (entity.APIToken, error) {
			stored = token

			return token, nil
		})

	minted, err := h.service.Mint(ctx, service.MintAPITokenInput{
		Name:   "CI",
		Scopes: readScopes(),
		Grants: wholeWorkspace(workspaceID),
	})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	if !strings.HasPrefix(minted.Value, entity.APITokenPrefix) {
		t.Fatalf(
			"token %q does not carry the %q prefix that makes a leak greppable",
			minted.Value, entity.APITokenPrefix,
		)
	}

	if strings.Contains(string(stored.TokenHash), minted.Value) {
		t.Fatal("the stored hash contains the plaintext token")
	}

	if string(stored.TokenHash) != string(entity.HashAPIToken(minted.Value)) {
		t.Fatal("the stored hash is not the hash of the returned token")
	}
}

func TestOneTokenMayReachSeveralWorkspaces(t *testing.T) {
	h := newHarness(t)

	first, second := uuid.New(), uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		first:  {role: entity.MembershipRoleAdmin, allTeams: true},
		second: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	var stored entity.APIToken

	h.tokens.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token entity.APIToken) (entity.APIToken, error) {
			stored = token

			return token, nil
		})

	if _, err := h.service.Mint(ctx, service.MintAPITokenInput{
		Name:   "CI",
		Scopes: readScopes(),
		Grants: entity.APITokenGrants{
			{WorkspaceID: first, AllTeams: true},
			{WorkspaceID: second, AllTeams: true},
		},
	}); err != nil {
		t.Fatalf("Mint: %v", err)
	}

	if !stored.Grants.Covers(first) || !stored.Grants.Covers(second) {
		t.Fatal("a token granted two workspaces did not carry both")
	}
}

func TestAGrantOnAWorkspaceTheCreatorIsNotInIsRefused(t *testing.T) {
	h := newHarness(t)

	mine := uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		mine: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	_, err := h.service.Mint(ctx, service.MintAPITokenInput{
		Name:   "CI",
		Scopes: readScopes(),
		Grants: entity.APITokenGrants{
			{WorkspaceID: mine, AllTeams: true},
			{WorkspaceID: uuid.New(), AllTeams: true},
		},
	})

	if !errors.Is(err, entity.ErrAPITokenGrantInvalid) {
		t.Fatalf("Mint error = %v, want ErrAPITokenGrantInvalid", err)
	}
}

func TestAGrantNamingATeamTheCreatorCannotSeeIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID, visible := uuid.New(), uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		workspaceID: {role: entity.MembershipRoleMember, teams: []uuid.UUID{visible}},
	})

	_, err := h.service.Mint(ctx, service.MintAPITokenInput{
		Name:   "CI",
		Scopes: readScopes(),
		Grants: entity.APITokenGrants{{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{uuid.New()}}},
	})

	if !errors.Is(err, entity.ErrAPITokenGrantInvalid) {
		t.Fatalf(
			"Mint error = %v, want ErrAPITokenGrantInvalid. Naming a team you cannot see would "+
				"let a token be pointed at work its creator has no access to.",
			err,
		)
	}
}

func TestScopesMustBeWithinReachInEveryGrantedWorkspace(t *testing.T) {
	h := newHarness(t)

	admin, member := uuid.New(), uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		admin:  {role: entity.MembershipRoleAdmin, allTeams: true},
		member: {role: entity.MembershipRoleMember, allTeams: true},
	})

	_, err := h.service.Mint(ctx, service.MintAPITokenInput{
		Name:   "CI",
		Scopes: manageScopes(),
		Grants: entity.APITokenGrants{
			{WorkspaceID: admin, AllTeams: true},
			{WorkspaceID: member, AllTeams: true},
		},
	})

	if !errors.Is(err, entity.ErrAPITokenScopeExceeds) {
		t.Fatalf(
			"Mint error = %v, want ErrAPITokenScopeExceeds. Adding a workspace where the creator "+
				"may do less must refuse outright rather than silently drop the scope.",
			err,
		)
	}
}

func TestATokenMayNotExceedItsCreatorsRights(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		workspaceID: {role: entity.MembershipRoleMember, allTeams: true},
	})

	_, err := h.service.Mint(ctx, service.MintAPITokenInput{
		Name:   "CI",
		Scopes: manageScopes(),
		Grants: wholeWorkspace(workspaceID),
	})

	if !errors.Is(err, entity.ErrAPITokenScopeExceeds) {
		t.Fatalf("Mint error = %v, want ErrAPITokenScopeExceeds", err)
	}
}

func TestATokenMayNotMintAnotherToken(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.ActorKindToken, map[uuid.UUID]access{
		workspaceID: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	_, err := h.service.Mint(ctx, service.MintAPITokenInput{
		Name:   "CI",
		Scopes: readScopes(),
		Grants: wholeWorkspace(workspaceID),
	})

	if !errors.Is(err, entity.ErrAPITokenMintForbidden) {
		t.Fatalf("Mint error = %v, want ErrAPITokenMintForbidden", err)
	}
}

func TestATokenMayNotRevokeAnotherToken(t *testing.T) {
	h := newHarness(t)

	ctx := h.actingAs(entity.ActorKindToken, nil)

	if err := h.service.Revoke(ctx, uuid.New()); !errors.Is(err, entity.ErrAPITokenMintForbidden) {
		t.Fatalf("Revoke error = %v, want ErrAPITokenMintForbidden", err)
	}
}

func TestSomebodyElsesTokenCannotBeRevokedByIdentifier(t *testing.T) {
	h := newHarness(t)

	ctx := h.actingAs(entity.ActorKindUser, nil)

	h.tokens.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		Return(entity.APIToken{ID: uuid.New(), AccountID: uuid.New()}, nil)

	if err := h.service.Revoke(ctx, uuid.New()); !errors.Is(err, entity.ErrAPITokenNotFound) {
		t.Fatalf(
			"Revoke error = %v, want ErrAPITokenNotFound. Holding somebody else's token id must "+
				"not be enough to kill it.",
			err,
		)
	}
}

func TestAnAdministratorMayOnlyRevokeATokenThatReachesTheirWorkspace(t *testing.T) {
	h := newHarness(t)

	mine, elsewhere := uuid.New(), uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		mine: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	h.tokens.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		Return(entity.APIToken{ID: uuid.New(), Grants: wholeWorkspace(elsewhere)}, nil)

	if err := h.service.RevokeInWorkspace(ctx, mine, uuid.New()); !errors.Is(err, entity.ErrAPITokenNotFound) {
		t.Fatalf(
			"RevokeInWorkspace error = %v, want ErrAPITokenNotFound. Administering one workspace "+
				"must not become an oracle over every token on the instance.",
			err,
		)
	}
}

func TestAnUnknownScopeIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		workspaceID: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	_, err := h.service.Mint(ctx, service.MintAPITokenInput{
		Name:   "CI",
		Scopes: entity.APIScopeSet{"team:destroy"},
		Grants: wholeWorkspace(workspaceID),
	})

	if !errors.Is(err, entity.ErrAPITokenScopeInvalid) {
		t.Fatalf("Mint error = %v, want ErrAPITokenScopeInvalid", err)
	}
}

func TestATokenWithNoScopesIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		workspaceID: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	_, err := h.service.Mint(ctx, service.MintAPITokenInput{
		Name:   "CI",
		Grants: wholeWorkspace(workspaceID),
	})

	if !errors.Is(err, entity.ErrAPITokenScopeInvalid) {
		t.Fatalf("Mint error = %v, want ErrAPITokenScopeInvalid", err)
	}
}

func TestATokenReachingNoWorkspaceIsRefused(t *testing.T) {
	h := newHarness(t)

	ctx := h.actingAs(entity.ActorKindUser, nil)

	_, err := h.service.Mint(ctx, service.MintAPITokenInput{Name: "CI", Scopes: readScopes()})

	if !errors.Is(err, entity.ErrAPITokenGrantMissing) {
		t.Fatalf("Mint error = %v, want ErrAPITokenGrantMissing", err)
	}
}

func storedToken(accountID uuid.UUID, scopes entity.APIScopeSet, grants entity.APITokenGrants) entity.APIToken {
	return entity.APIToken{
		ID:        uuid.New(),
		AccountID: accountID,
		Name:      "CI",
		TokenHash: entity.HashAPIToken("nrn_value"),
		Scopes:    scopes,
		Grants:    grants,
	}
}

func TestAnAuthenticatedTokenCarriesItsGrantsAndScopes(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	accountID := uuid.New()
	token := storedToken(accountID, readScopes(), wholeWorkspace(workspaceID))

	h.tokens.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(token, nil)
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

	if !actor.ConfinedTo(workspaceID) {
		t.Fatal("a token actor was not allowed into the workspace it was granted")
	}

	if actor.ConfinedTo(uuid.New()) {
		t.Fatal("a token actor was allowed into a workspace it was never granted")
	}

	if actor.Holds(entity.NewPermission(entity.ResourceTeam, entity.ActionManage)) {
		t.Fatal("the actor holds a permission its token was never granted")
	}

	if actor.TokenName != token.Name {
		t.Fatalf("actor token name = %q, want %q so activity can name the credential", actor.TokenName, token.Name)
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
			token := mutate(storedToken(uuid.New(), readScopes(), wholeWorkspace(uuid.New())))

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

	accountID := uuid.New()
	token := storedToken(accountID, readScopes(), wholeWorkspace(uuid.New()))

	h.tokens.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(token, nil)
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

	accountID := uuid.New()
	token := storedToken(accountID, readScopes(), wholeWorkspace(uuid.New()))
	justNow := time.Now().UTC()
	token.LastUsedAt = &justNow

	h.tokens.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(token, nil)
	h.accounts.EXPECT().
		GetByID(gomock.Any(), accountID).
		Return(entity.Account{ID: accountID, Status: entity.AccountStatusActive}, nil)

	if _, err := h.service.Authenticate(context.Background(), "nrn_value"); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
}

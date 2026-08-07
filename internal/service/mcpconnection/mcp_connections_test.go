package mcpconnection_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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
	mcpauthstaterepo "github.com/usenorn/norn/internal/repository/mcpauthstate"
	mcpclientrepo "github.com/usenorn/norn/internal/repository/mcpclient"
	mcpconnectionrepo "github.com/usenorn/norn/internal/repository/mcpconnection"
	mcptokenrepo "github.com/usenorn/norn/internal/repository/mcptoken"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	mcpconnectionsvc "github.com/usenorn/norn/internal/service/mcpconnection"
)

type access struct {
	role     entity.MembershipRole
	allTeams bool
	teams    []uuid.UUID
}

type harness struct {
	clients     *mcpclientrepo.MockMCPClient
	connections *mcpconnectionrepo.MockMCPConnection
	tokens      *mcptokenrepo.MockMCPToken
	authState   *mcpauthstaterepo.MockMCPAuthState
	accounts    *accountrepo.MockAccount
	workspaces  *workspacerepo.MockWorkspace
	memberships *membershiprepo.MockMembership
	authorizer  *authorizersvc.MockAuthorizer
	service     service.MCPConnections

	accountID uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		clients:     mcpclientrepo.NewMockMCPClient(ctrl),
		connections: mcpconnectionrepo.NewMockMCPConnection(ctrl),
		tokens:      mcptokenrepo.NewMockMCPToken(ctrl),
		authState:   mcpauthstaterepo.NewMockMCPAuthState(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		workspaces:  workspacerepo.NewMockWorkspace(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		accountID:   uuid.New(),
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	audit := auditsvc.NewMockAudit(ctrl)
	audit.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()

	h.service = mcpconnectionsvc.New(
		h.clients,
		h.connections,
		h.tokens,
		h.authState,
		h.accounts,
		h.workspaces,
		h.memberships,
		h.authorizer,
		audit,
		transactor,
		config.App{BaseURL: "https://norn.test"},
		config.MCP{
			Enabled:         true,
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 90 * 24 * time.Hour,
			AuthRequestTTL:  10 * time.Minute,
			AuthCodeTTL:     2 * time.Minute,
		},
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

func challengeFor(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func liveConnection(h *harness, capability entity.MCPCapability) entity.MCPConnection {
	return entity.MCPConnection{
		ID:         uuid.New(),
		AccountID:  h.accountID,
		ClientID:   uuid.New(),
		ClientName: "Claude",
		Scopes:     entity.MCPScopesFor(capability),
	}
}

func TestAuthorizationRefusesAnUnregisteredRedirect(t *testing.T) {
	h := newHarness(t)
	clientID := uuid.New()

	h.clients.EXPECT().
		GetByID(gomock.Any(), clientID).
		Return(entity.MCPClient{
			ID:           clientID,
			Name:         "probe",
			RedirectURIs: []string{"http://127.0.0.1:9000/cb"},
		}, nil)

	_, err := h.service.BeginAuthorization(context.Background(), service.BeginMCPAuthorizationInput{
		ClientID:            clientID.String(),
		RedirectURI:         "https://attacker.example/cb",
		ResponseType:        "code",
		Scope:               "read",
		CodeChallenge:       challengeFor("verifier"),
		CodeChallengeMethod: "S256",
	})

	if !errors.Is(err, entity.ErrMCPRedirectInvalid) {
		t.Fatalf("an unregistered redirect produced %v, want ErrMCPRedirectInvalid", err)
	}
}

func TestAuthorizationRequiresPKCE(t *testing.T) {
	h := newHarness(t)
	clientID := uuid.New()

	h.clients.EXPECT().
		GetByID(gomock.Any(), clientID).
		Return(entity.MCPClient{
			ID:           clientID,
			RedirectURIs: []string{"http://127.0.0.1:9000/cb"},
		}, nil).
		AnyTimes()

	for _, probe := range []struct {
		challenge string
		method    string
	}{
		{"", "S256"},
		{challengeFor("verifier"), ""},
		{challengeFor("verifier"), "plain"},
	} {
		_, err := h.service.BeginAuthorization(context.Background(), service.BeginMCPAuthorizationInput{
			ClientID:            clientID.String(),
			RedirectURI:         "http://127.0.0.1:9000/cb",
			ResponseType:        "code",
			Scope:               "read",
			CodeChallenge:       probe.challenge,
			CodeChallengeMethod: probe.method,
		})

		if !errors.Is(err, entity.ErrMCPRequestInvalid) {
			t.Errorf(
				"challenge %q with method %q produced %v, want ErrMCPRequestInvalid",
				probe.challenge, probe.method, err,
			)
		}
	}
}

func TestExchangeRefusesAWrongPKCEVerifier(t *testing.T) {
	h := newHarness(t)
	clientID := uuid.New()

	h.authState.EXPECT().
		TakeCode(gomock.Any(), "the-code").
		Return(entity.MCPAuthCode{
			ClientID:      clientID,
			AccountID:     h.accountID,
			RedirectURI:   "http://127.0.0.1:9000/cb",
			Capability:    entity.MCPCapabilityWrite,
			CodeChallenge: challengeFor("right-verifier"),
		}, nil)

	_, err := h.service.Exchange(context.Background(), service.ExchangeMCPCodeInput{
		ClientID:     clientID.String(),
		Code:         "the-code",
		RedirectURI:  "http://127.0.0.1:9000/cb",
		CodeVerifier: "wrong-verifier",
	})

	if !errors.Is(err, entity.ErrMCPCodeInvalid) {
		t.Fatalf("a wrong verifier produced %v, want ErrMCPCodeInvalid", err)
	}
}

func TestExchangeMintsAConnectionThatFollowsMembership(t *testing.T) {
	h := newHarness(t)
	clientID := uuid.New()
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

	h.authState.EXPECT().
		TakeCode(gomock.Any(), "the-code").
		Return(entity.MCPAuthCode{
			ClientID:      clientID,
			AccountID:     h.accountID,
			RedirectURI:   "http://127.0.0.1:9000/cb",
			Capability:    entity.MCPCapabilityWrite,
			CodeChallenge: challengeFor(verifier),
		}, nil)

	h.clients.EXPECT().
		GetByID(gomock.Any(), clientID).
		Return(entity.MCPClient{ID: clientID, Name: "Claude"}, nil)

	var stored entity.MCPConnection

	h.connections.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, connection entity.MCPConnection) (entity.MCPConnection, error) {
			connection.ID = uuid.New()
			stored = connection

			return connection, nil
		})

	minted := make(map[entity.MCPTokenKind]entity.MCPToken, 2)

	h.tokens.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token entity.MCPToken) (entity.MCPToken, error) {
			minted[token.Kind] = token

			return token, nil
		}).
		Times(2)

	pair, err := h.service.Exchange(context.Background(), service.ExchangeMCPCodeInput{
		ClientID:     clientID.String(),
		Code:         "the-code",
		RedirectURI:  "http://127.0.0.1:9000/cb",
		CodeVerifier: verifier,
	})
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}

	if stored.Grants != nil {
		t.Fatal(
			"a fresh connection carried grants; it must follow the owner's membership so new " +
				"workspaces are reachable without re-authorization",
		)
	}

	if stored.ClientName != "Claude" {
		t.Errorf("connection client name = %q, want the registered client name", stored.ClientName)
	}

	if !stored.Scopes.Permits(entity.ResourceIssue, entity.ActionManage) {
		t.Error("a write consent did not grant issue management")
	}

	if !strings.HasPrefix(pair.AccessToken, entity.MCPTokenPrefix) ||
		!strings.HasPrefix(pair.RefreshToken, entity.MCPTokenPrefix) {
		t.Error("minted token values do not carry the mcp prefix")
	}

	access, ok := minted[entity.MCPTokenKindAccess]
	if !ok || string(access.TokenHash) != string(entity.HashMCPToken(pair.AccessToken)) {
		t.Error("the stored access hash is not the hash of the returned access token")
	}

	refresh, ok := minted[entity.MCPTokenKindRefresh]
	if !ok || string(refresh.TokenHash) != string(entity.HashMCPToken(pair.RefreshToken)) {
		t.Error("the stored refresh hash is not the hash of the returned refresh token")
	}

	if pair.ExpiresIn != int(time.Hour.Seconds()) {
		t.Errorf("ExpiresIn = %d, want %d", pair.ExpiresIn, int(time.Hour.Seconds()))
	}
}

func TestRefreshRotatesThePair(t *testing.T) {
	h := newHarness(t)
	connection := liveConnection(h, entity.MCPCapabilityRead)
	tokenID := uuid.New()

	h.tokens.EXPECT().
		GetByHash(gomock.Any(), gomock.Any()).
		Return(entity.MCPToken{
			ID:           tokenID,
			ConnectionID: connection.ID,
			Kind:         entity.MCPTokenKindRefresh,
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil)

	h.connections.EXPECT().GetByID(gomock.Any(), connection.ID).Return(connection, nil)
	h.tokens.EXPECT().Consume(gomock.Any(), tokenID, gomock.Any()).Return(nil)
	h.tokens.EXPECT().PruneExpired(gomock.Any(), connection.ID, gomock.Any()).Return(nil)
	h.tokens.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token entity.MCPToken) (entity.MCPToken, error) {
			return token, nil
		}).
		Times(2)

	pair, err := h.service.Refresh(context.Background(), service.RefreshMCPTokenInput{
		RefreshToken: "nmcp_old",
		ClientID:     connection.ClientID.String(),
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("rotation returned an empty pair")
	}
}

func TestARefreshTokenIsOnlySpentByTheClientItWasIssuedTo(t *testing.T) {
	h := newHarness(t)
	connection := liveConnection(h, entity.MCPCapabilityRead)

	h.tokens.EXPECT().
		GetByHash(gomock.Any(), gomock.Any()).
		Return(entity.MCPToken{
			ID:           uuid.New(),
			ConnectionID: connection.ID,
			Kind:         entity.MCPTokenKindRefresh,
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil).
		Times(2)

	h.connections.EXPECT().GetByID(gomock.Any(), connection.ID).Return(connection, nil).Times(2)

	for name, clientID := range map[string]string{
		"another client": uuid.NewString(),
		"no client":      "",
	} {
		_, err := h.service.Refresh(context.Background(), service.RefreshMCPTokenInput{
			RefreshToken: "nmcp_old",
			ClientID:     clientID,
		})

		if !errors.Is(err, entity.ErrMCPCodeInvalid) {
			t.Errorf(
				"%s: refreshing gave %v. These clients hold no secret, so naming the client is the "+
					"only thing binding a refresh token to the one it was issued to.",
				name, err,
			)
		}
	}
}

func TestAReplayedRefreshTokenRevokesTheWholeConnection(t *testing.T) {
	h := newHarness(t)
	connectionID := uuid.New()
	consumedAt := time.Now().Add(-time.Minute)

	h.tokens.EXPECT().
		GetByHash(gomock.Any(), gomock.Any()).
		Return(entity.MCPToken{
			ID:           uuid.New(),
			ConnectionID: connectionID,
			Kind:         entity.MCPTokenKindRefresh,
			ExpiresAt:    time.Now().Add(time.Hour),
			ConsumedAt:   &consumedAt,
		}, nil)

	h.connections.EXPECT().
		GetByID(gomock.Any(), connectionID).
		Return(entity.MCPConnection{ID: connectionID, ClientName: "Claude"}, nil)
	h.connections.EXPECT().Revoke(gomock.Any(), connectionID, gomock.Any()).Return(nil)
	h.tokens.EXPECT().DeleteForConnection(gomock.Any(), connectionID).Return(nil)

	_, err := h.service.Refresh(context.Background(), service.RefreshMCPTokenInput{
		RefreshToken: "nmcp_replayed",
	})

	if !errors.Is(err, entity.ErrMCPCodeInvalid) {
		t.Fatalf("a replayed refresh token produced %v, want ErrMCPCodeInvalid", err)
	}
}

func TestAuthenticateRefusesARevokedConnectionImmediately(t *testing.T) {
	h := newHarness(t)
	revokedAt := time.Now()
	connection := liveConnection(h, entity.MCPCapabilityRead)
	connection.RevokedAt = &revokedAt

	h.tokens.EXPECT().
		GetByHash(gomock.Any(), gomock.Any()).
		Return(entity.MCPToken{
			ID:           uuid.New(),
			ConnectionID: connection.ID,
			Kind:         entity.MCPTokenKindAccess,
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil)

	h.connections.EXPECT().GetByID(gomock.Any(), connection.ID).Return(connection, nil)

	_, _, err := h.service.Authenticate(context.Background(), "nmcp_live")

	if !errors.Is(err, entity.ErrMCPConnectionRevoked) {
		t.Fatalf(
			"an access token whose connection was revoked produced %v; revocation must take "+
				"effect on the next operation, not at token expiry",
			err,
		)
	}
}

func TestAuthenticateBuildsAnUnconfinedTokenActor(t *testing.T) {
	h := newHarness(t)
	connection := liveConnection(h, entity.MCPCapabilityWrite)

	h.tokens.EXPECT().
		GetByHash(gomock.Any(), gomock.Any()).
		Return(entity.MCPToken{
			ID:           uuid.New(),
			ConnectionID: connection.ID,
			Kind:         entity.MCPTokenKindAccess,
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil)

	h.connections.EXPECT().GetByID(gomock.Any(), connection.ID).Return(connection, nil)
	h.accounts.EXPECT().
		GetByID(gomock.Any(), h.accountID).
		Return(entity.Account{ID: h.accountID, Status: entity.AccountStatusActive}, nil)
	h.connections.EXPECT().RecordUsage(gomock.Any(), connection.ID, gomock.Any()).Return(nil)

	actor, _, err := h.service.Authenticate(context.Background(), "nmcp_live")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if actor.Kind != entity.ActorKindToken {
		t.Errorf("actor kind = %q, want token", actor.Kind)
	}

	if actor.ConnectionID == nil || *actor.ConnectionID != connection.ID {
		t.Error("the actor does not carry its connection identity")
	}

	if actor.ConfinedTo(uuid.New()) != true {
		t.Fatal(
			"a fresh connection actor was confined; nil grants must leave confinement to the " +
				"live membership lookup so every workspace the owner joins is reachable",
		)
	}

	if !actor.Holds(entity.NewPermission(entity.ResourceIssue, entity.ActionManage)) {
		t.Error("a write connection cannot manage issues")
	}

	if actor.Holds(entity.NewPermission(entity.ResourceMembership, entity.ActionManage)) {
		t.Error("a connection actor may manage memberships")
	}
}

func TestAnAccessTokenIsNotARefreshTokenAndViceVersa(t *testing.T) {
	h := newHarness(t)

	h.tokens.EXPECT().
		GetByHash(gomock.Any(), gomock.Any()).
		Return(entity.MCPToken{
			ID:        uuid.New(),
			Kind:      entity.MCPTokenKindRefresh,
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil)

	if _, _, err := h.service.Authenticate(context.Background(), "nmcp_refresh"); !errors.Is(
		err, entity.ErrMCPTokenNotFound,
	) {
		t.Fatalf("a refresh token authenticated as an access token: %v", err)
	}

	h.tokens.EXPECT().
		GetByHash(gomock.Any(), gomock.Any()).
		Return(entity.MCPToken{
			ID:        uuid.New(),
			Kind:      entity.MCPTokenKindAccess,
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil)

	if _, err := h.service.Refresh(context.Background(), service.RefreshMCPTokenInput{
		RefreshToken: "nmcp_access",
	}); !errors.Is(err, entity.ErrMCPCodeInvalid) {
		t.Fatalf("an access token refreshed the pair: %v", err)
	}
}

func TestNarrowingNeverWidens(t *testing.T) {
	h := newHarness(t)

	reachable, hidden := uuid.New(), uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		reachable: {role: entity.MembershipRoleMember, allTeams: true},
	})

	write := entity.MCPCapabilityWrite

	readOnly := liveConnection(h, entity.MCPCapabilityRead)
	h.connections.EXPECT().GetByID(gomock.Any(), readOnly.ID).Return(readOnly, nil)

	if _, err := h.service.Narrow(ctx, readOnly.ID, service.NarrowMCPConnectionInput{
		Capability: &write,
	}); !errors.Is(err, entity.ErrMCPGrantInvalid) {
		t.Fatalf("widening read to write produced %v, want ErrMCPGrantInvalid", err)
	}

	unnarrowed := liveConnection(h, entity.MCPCapabilityRead)
	h.connections.EXPECT().GetByID(gomock.Any(), unnarrowed.ID).Return(unnarrowed, nil)

	if _, err := h.service.Narrow(ctx, unnarrowed.ID, service.NarrowMCPConnectionInput{
		Grants: &entity.APITokenGrants{{WorkspaceID: hidden, AllTeams: true}},
	}); !errors.Is(err, entity.ErrMCPGrantInvalid) {
		t.Fatalf(
			"narrowing to an unreachable workspace produced %v; the failure must be "+
				"indistinguishable from an invalid grant so workspace existence never leaks",
			err,
		)
	}

	narrowed := liveConnection(h, entity.MCPCapabilityRead)
	narrowed.Grants = entity.APITokenGrants{{WorkspaceID: reachable, AllTeams: true}}
	h.connections.EXPECT().GetByID(gomock.Any(), narrowed.ID).Return(narrowed, nil)

	if _, err := h.service.Narrow(ctx, narrowed.ID, service.NarrowMCPConnectionInput{
		Grants: &entity.APITokenGrants{
			{WorkspaceID: reachable, AllTeams: true},
			{WorkspaceID: hidden, AllTeams: true},
		},
	}); !errors.Is(err, entity.ErrMCPGrantInvalid) {
		t.Fatalf("adding a workspace to a narrowed connection produced %v", err)
	}
}

func TestNarrowingToAWorkspaceSubsetSticks(t *testing.T) {
	h := newHarness(t)

	kept := uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		kept: {role: entity.MembershipRoleMember, allTeams: true},
	})

	connection := liveConnection(h, entity.MCPCapabilityWrite)
	h.connections.EXPECT().GetByID(gomock.Any(), connection.ID).Return(connection, nil)

	replaced := entity.APITokenGrants{{WorkspaceID: kept, AllTeams: true}}

	h.connections.EXPECT().
		ReplaceGrants(gomock.Any(), connection.ID, replaced).
		Return(nil)

	updated := connection
	updated.Grants = replaced
	h.connections.EXPECT().GetByID(gomock.Any(), connection.ID).Return(updated, nil)

	result, err := h.service.Narrow(ctx, connection.ID, service.NarrowMCPConnectionInput{
		Grants: &replaced,
	})
	if err != nil {
		t.Fatalf("Narrow: %v", err)
	}

	if result.FollowsMembership() {
		t.Fatal("a narrowed connection still follows membership")
	}

	if !result.Grants.Covers(kept) {
		t.Error("the kept workspace is not covered after narrowing")
	}
}

func TestSomeoneElsesConnectionIsInvisibleToNarrowAndRevoke(t *testing.T) {
	h := newHarness(t)
	ctx := h.actingAs(entity.ActorKindUser, nil)

	foreign := entity.MCPConnection{
		ID:        uuid.New(),
		AccountID: uuid.New(),
		Scopes:    entity.MCPScopesFor(entity.MCPCapabilityRead),
	}

	h.connections.EXPECT().GetByID(gomock.Any(), foreign.ID).Return(foreign, nil).Times(2)

	if err := h.service.Revoke(ctx, foreign.ID); !errors.Is(err, entity.ErrMCPConnectionNotFound) {
		t.Fatalf("revoking a foreign connection produced %v, want not-found", err)
	}

	read := entity.MCPCapabilityRead

	if _, err := h.service.Narrow(ctx, foreign.ID, service.NarrowMCPConnectionInput{
		Capability: &read,
	}); !errors.Is(err, entity.ErrMCPConnectionNotFound) {
		t.Fatalf("narrowing a foreign connection produced %v, want not-found", err)
	}
}

func TestWorkspaceRevocationConcealsConnectionsBeyondItsReach(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.ActorKindUser, map[uuid.UUID]access{
		workspaceID: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	elsewhere := entity.MCPConnection{
		ID:        uuid.New(),
		AccountID: uuid.New(),
		Scopes:    entity.MCPScopesFor(entity.MCPCapabilityRead),
		Grants:    entity.APITokenGrants{{WorkspaceID: uuid.New(), AllTeams: true}},
	}

	h.connections.EXPECT().GetByID(gomock.Any(), elsewhere.ID).Return(elsewhere, nil)

	err := h.service.RevokeInWorkspace(ctx, workspaceID, elsewhere.ID)

	if !errors.Is(err, entity.ErrMCPConnectionNotFound) {
		t.Fatalf(
			"revoking a connection narrowed away from the workspace produced %v; the admin must "+
				"not learn the connection exists",
			err,
		)
	}
}

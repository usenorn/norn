package ssoconnection_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	connectionrepo "github.com/usenorn/norn/internal/repository/oidcconnection"
	providerrepo "github.com/usenorn/norn/internal/repository/oidcprovider"
	staterepo "github.com/usenorn/norn/internal/repository/oidcstate"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
	ssosvc "github.com/usenorn/norn/internal/service/ssoconnection"
)

type harness struct {
	connections *connectionrepo.MockOIDCConnection
	states      *staterepo.MockOIDCState
	workspaces  *workspacerepo.MockWorkspace
	accounts    *accountrepo.MockAccount
	memberships *membershiprepo.MockMembership
	provider    *providerrepo.MockOIDCProvider
	sessions    *sessionsvc.MockSessions
	authorizer  *authorizersvc.MockAuthorizer
	transactor  *transactorrepo.MockTransactor
	service     service.SSOConnections
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		connections: connectionrepo.NewMockOIDCConnection(ctrl),
		states:      staterepo.NewMockOIDCState(ctrl),
		workspaces:  workspacerepo.NewMockWorkspace(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		provider:    providerrepo.NewMockOIDCProvider(ctrl),
		sessions:    sessionsvc.NewMockSessions(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = ssosvc.New(
		h.connections,
		h.states,
		h.workspaces,
		h.accounts,
		h.memberships,
		h.provider,
		h.sessions,
		h.authorizer,
		h.transactor,
	)

	return h
}

func (h *harness) allow(workspaceID uuid.UUID) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
			Scope: entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true},
		}, nil).
		AnyTimes()
}

func connection(workspaceID uuid.UUID, provisioning bool) entity.OIDCConnection {
	return entity.OIDCConnection{
		WorkspaceID: workspaceID,
		Endpoints: entity.OIDCEndpoints{
			Issuer:                "https://login.example.com",
			AuthorizationEndpoint: "https://login.example.com/auth",
			TokenEndpoint:         "https://login.example.com/token",
			JWKSURI:               "https://login.example.com/certs",
		},
		Discovered:   true,
		ClientID:     "norn",
		ClientSecret: "s3cr3t",
		Scopes:       entity.DefaultOIDCScopes,
		Provisioning: provisioning,
	}
}

func verifiedClaims(email string) entity.OIDCClaims {
	verified := true

	return entity.OIDCClaims{
		Subject:       "provider-subject",
		Email:         email,
		EmailVerified: &verified,
		Name:          "Ada Lovelace",
	}
}

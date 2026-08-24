package scm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
	"github.com/usenorn/norn/internal/service"
	auditmock "github.com/usenorn/norn/internal/service/audit"
	authorizermock "github.com/usenorn/norn/internal/service/authorizer"
)

type addHarness struct {
	service     *connections
	connections *scmrepo.MockSCMConnection
	hub         *MockForge
	cache       *forge.Credentials
	connection  entity.SCMConnection
	workspace   uuid.UUID
	spent       string
}

func adding(t *testing.T) *addHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	connectionRepository := scmrepo.NewMockSCMConnection(ctrl)
	apps := scmrepo.NewMockSCMApp(ctrl)
	forgeApp := NewMockForgeApp(ctrl)
	forges := NewMockForges(ctrl)
	hub := NewMockForge(ctrl)
	authorizer := authorizermock.NewMockAuthorizer(ctrl)
	audit := auditmock.NewMockAudit(ctrl)
	cache := forge.NewCredentials()

	workspace := uuid.New()
	appID := uuid.New()

	connection := entity.SCMConnection{
		ID:             uuid.New(),
		WorkspaceID:    workspace,
		Provider:       entity.SCMProviderGitHub,
		AuthKind:       entity.SCMAuthApp,
		AppID:          appID,
		InstallationID: "884411",
		Label:          "flagroll",
	}

	forges.EXPECT().App(entity.SCMProviderGitHub).Return(forgeApp, nil).AnyTimes()
	forges.EXPECT().Lookup(entity.SCMProviderGitHub).Return(hub, nil).AnyTimes()
	audit.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()

	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Role:      entity.MembershipRoleAdmin,
			Actor:     entity.Actor{AccountID: uuid.New()},
			Workspace: entity.Workspace{ID: workspace, Slug: "northwind"},
		}, nil).
		AnyTimes()

	connectionRepository.EXPECT().
		GetByID(gomock.Any(), workspace, connection.ID).
		Return(connection, nil).
		AnyTimes()

	apps.EXPECT().
		Secrets(gomock.Any(), appID).
		Return(entity.SCMApp{ID: appID, ExternalAppID: "4711"}, nil).
		AnyTimes()

	forgeApp.EXPECT().
		MintInstallationToken(gomock.Any(), gomock.Any(), "884411").
		Return(entity.SCMCredential{
			Token:     "ghs-after-the-grant",
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil).
		Times(1)

	return &addHarness{
		service: &connections{
			connections: connectionRepository,
			apps:        apps,
			forges:      forges,
			authorizer:  authorizer,
			audit:       audit,
			credentials: newCredentials(connectionRepository, apps, forges, cache),
		},
		connections: connectionRepository,
		hub:         hub,
		cache:       cache,
		connection:  connection,
		workspace:   workspace,
	}
}

func (h *addHarness) add(refused error) error {
	h.hub.EXPECT().
		Repository(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, target entity.SCMTarget) (entity.SCMRemoteRepository, error) {
			h.spent = target.Token

			return entity.SCMRemoteRepository{}, refused
		})

	_, err := h.service.AddRepository(context.Background(), h.workspace, service.AddRepositoryInput{
		ConnectionID: h.connection.ID,
		FullName:     "flagroll/ledger",
	})

	return err
}

func unreachable() error {
	return entity.SCMRepositoryUnreachableError{
		Provider:   entity.SCMProviderGitHub,
		Repository: "flagroll/ledger",
		Reason:     "the repository is not visible to this token",
	}
}

func TestConnectingARepositoryAsksWithATokenThatCanAlreadySeeIt(t *testing.T) {
	held := adding(t)

	held.cache.Put(held.connection.ID, entity.SCMCredential{
		Token:     "ghs-before-the-grant",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	if err := held.add(unreachable()); err == nil {
		t.Fatal("a repository the installation cannot reach was connected anyway")
	}

	if held.spent != "ghs-after-the-grant" {
		t.Fatalf(
			"the repository was asked for with %q. An installation token carries the "+
				"repositories the installation reached when it was minted, so connecting one "+
				"granted since would be refused for the rest of that token's hour",
			held.spent,
		)
	}
}

func TestARepositoryTheInstallationCannotReachDoesNotBreakTheConnection(t *testing.T) {
	held := adding(t)

	if err := held.add(unreachable()); !errors.As(err, &entity.SCMRepositoryUnreachableError{}) {
		t.Fatalf(
			"connecting a repository came back with %v, want the forge's own refusal passed "+
				"through untouched",
			err,
		)
	}
}

func TestACredentialTheForgeRefusesBreaksTheConnection(t *testing.T) {
	held := adding(t)

	held.connections.EXPECT().
		MarkBroken(
			gomock.Any(),
			held.connection.ID,
			entity.SCMBrokenCredentialsRejected,
			gomock.Any(),
			gomock.Any(),
		).
		Return(nil).
		Times(1)

	err := held.add(entity.SCMCredentialsRejectedError{
		Provider: entity.SCMProviderGitHub,
		Reason:   "bad credentials",
	})
	if err == nil {
		t.Fatal("a connection the forge no longer accepts connected a repository anyway")
	}
}

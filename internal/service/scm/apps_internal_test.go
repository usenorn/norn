package scm

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
	statemock "github.com/usenorn/norn/internal/repository/scmappstate"
	"github.com/usenorn/norn/internal/service"
	authorizermock "github.com/usenorn/norn/internal/service/authorizer"
)

type appHarness struct {
	service     *apps
	registry    *scmrepo.MockSCMApp
	connections *scmrepo.MockSCMConnection
	states      *statemock.MockSCMAppState
	forgeApp    *MockForgeApp
	authorizer  *authorizermock.MockAuthorizer
	workspace   uuid.UUID
	actor       uuid.UUID
}

func appsFor(t *testing.T, cfg config.SourceControl) *appHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	registry := scmrepo.NewMockSCMApp(ctrl)
	connections := scmrepo.NewMockSCMConnection(ctrl)
	states := statemock.NewMockSCMAppState(ctrl)
	forgeApp := NewMockForgeApp(ctrl)
	forges := NewMockForges(ctrl)
	hub := NewMockForge(ctrl)
	authorizer := authorizermock.NewMockAuthorizer(ctrl)

	forges.EXPECT().App(entity.SCMProviderGitHub).Return(forgeApp, nil).AnyTimes()
	forges.EXPECT().Lookup(entity.SCMProviderGitHub).Return(hub, nil).AnyTimes()
	hub.EXPECT().Endpoint().Return("https://api.github.com").AnyTimes()

	workspace := uuid.New()
	actor := uuid.New()

	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Role:      entity.MembershipRoleAdmin,
			Actor:     entity.Actor{AccountID: actor},
			Workspace: entity.Workspace{ID: workspace, Slug: "northwind"},
		}, nil).
		AnyTimes()

	service := NewApps(registry, connections, states, forges, authorizer, cfg)

	return &appHarness{
		service:     service.(*apps),
		registry:    registry,
		connections: connections,
		states:      states,
		forgeApp:    forgeApp,
		authorizer:  authorizer,
		workspace:   workspace,
		actor:       actor,
	}
}

func cloudConfig() config.SourceControl {
	return config.SourceControl{
		GitHubAppID:            "4711",
		GitHubAppSlug:          "norn",
		GitHubAppClientID:      "Iv1.deadbeef",
		GitHubAppClientSecret:  "shhh",
		GitHubAppPrivateKey:    "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----",
		GitHubAppWebhookSecret: "hook",
	}
}

func TestTheCloudApplicationIsAdoptedFromConfigurationOnce(t *testing.T) {
	held := appsFor(t, cloudConfig())
	stored := entity.SCMApp{ID: uuid.New(), Provider: entity.SCMProviderGitHub, ExternalAppID: "4711"}

	first := held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, "https://api.github.com").
		Return(entity.SCMApp{}, entity.ErrSCMAppNotFound)

	held.registry.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input repository.SCMAppInput) (entity.SCMApp, error) {
			if input.PrivateKey == "" || input.WebhookSecret == "" {
				return entity.SCMApp{}, errors.New("the application was adopted without its keys")
			}

			return stored, nil
		})

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, "https://api.github.com").
		Return(stored, nil).
		After(first)

	held.registry.EXPECT().
		Secrets(gomock.Any(), gomock.Any()).
		Return(entity.SCMApp{}, nil).
		AnyTimes()

	for attempt := range 2 {
		found, err := held.service.Application(
			context.Background(),
			held.workspace,
			entity.SCMProviderGitHub,
		)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}

		if found.ID != stored.ID {
			t.Fatalf("attempt %d returned application %s, want %s", attempt, found.ID, stored.ID)
		}
	}
}

func TestAnInstanceThatWasHandedAnApplicationDoesNotRegisterAnother(t *testing.T) {
	held := appsFor(t, cloudConfig())

	_, err := held.service.Registration(context.Background(), registerInput(held.workspace))

	if !errors.Is(err, entity.ErrSCMAppExists) {
		t.Fatalf(
			"registering returned %v. This instance was handed an application in its own "+
				"configuration, so a second one would take over its webhooks and leave the first "+
				"signing deliveries nobody reads",
			err,
		)
	}
}

func TestARegistrationCannotBeFinishedAsAConnection(t *testing.T) {
	held := appsFor(t, config.SourceControl{})

	held.states.EXPECT().
		Take(gomock.Any(), "the-state").
		Return(entity.SCMAppState{
			Purpose:     entity.SCMAppConnect,
			Provider:    entity.SCMProviderGitHub,
			WorkspaceID: held.workspace,
		}, nil)

	_, err := held.service.CompleteRegistration(context.Background(), "the-code", "the-state")

	if !errors.Is(err, entity.ErrSCMAppStateNotFound) {
		t.Fatalf(
			"finishing a sign-in as a registration returned %v. The two exchanges answer on "+
				"different routes with the same one-shot token, so the purpose is the only thing "+
				"stopping one being spent on the other",
			err,
		)
	}
}

func TestAConnectionCannotBeFinishedAsARegistration(t *testing.T) {
	held := appsFor(t, config.SourceControl{})

	held.states.EXPECT().
		Take(gomock.Any(), "the-state").
		Return(entity.SCMAppState{
			Purpose:     entity.SCMAppRegister,
			Provider:    entity.SCMProviderGitHub,
			WorkspaceID: held.workspace,
		}, nil)

	_, err := held.service.CompleteAuthorization(
		context.Background(),
		"the-code",
		"the-state",
		"https://norn.example/v1/source-control/github-app/connected",
	)

	if !errors.Is(err, entity.ErrSCMAppStateNotFound) {
		t.Fatalf("finishing a registration as a sign-in returned %v, want the state sentinel", err)
	}
}

func TestWhatIsStashedForTheScreenCarriesNoCredential(t *testing.T) {
	held := appsFor(t, cloudConfig())
	registered := entity.SCMApp{
		ID:            uuid.New(),
		Provider:      entity.SCMProviderGitHub,
		ExternalAppID: "4711",
		ClientID:      "Iv1.deadbeef",
	}

	held.states.EXPECT().
		Take(gomock.Any(), "the-state").
		Return(entity.SCMAppState{
			Purpose:       entity.SCMAppConnect,
			Provider:      entity.SCMProviderGitHub,
			WorkspaceID:   held.workspace,
			WorkspaceSlug: "northwind",
		}, nil)

	sealed := registered
	sealed.ClientSecret = "the-client-secret"
	sealed.PrivateKey = "-----BEGIN RSA PRIVATE KEY-----"

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, gomock.Any()).
		Return(registered, nil)

	held.registry.EXPECT().
		Secrets(gomock.Any(), registered.ID).
		Return(sealed, nil)

	held.forgeApp.EXPECT().
		ExchangeCode(gomock.Any(), gomock.Any(), "the-code", gomock.Any()).
		DoAndReturn(func(
			_ context.Context, app entity.SCMApp, _, _ string,
		) (string, error) {
			if app.ClientSecret == "" {
				return "", errors.New(
					"the exchange was made without a client secret. The forge answers such a " +
						"call with no token at all, so the sign-in can never complete",
				)
			}

			return "gho-the-user-token", nil
		})

	held.forgeApp.EXPECT().
		Installations(gomock.Any(), gomock.Any(), "gho-the-user-token").
		Return([]entity.SCMInstallation{{ExternalID: "884411", AccountLogin: "flagroll"}}, nil)

	held.states.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, stash entity.SCMAppState) error {
			for _, installation := range stash.Installations {
				if installation.ExternalID == "" {
					return errors.New("an installation was stashed without its identifier")
				}
			}

			if stash.Purpose != entity.SCMAppChosen {
				return errors.New("the stash was written under the wrong purpose")
			}

			return nil
		})

	choice, err := held.service.CompleteAuthorization(
		context.Background(),
		"the-code",
		"the-state",
		"https://norn.example/v1/source-control/github-app/connected",
	)
	if err != nil {
		t.Fatalf("complete the sign-in: %v", err)
	}

	if choice.Handle == "" || len(choice.Installations) != 1 {
		t.Fatalf("the sign-in produced nothing to choose from: %+v", choice)
	}

	if choice.WorkspaceSlug != "northwind" {
		t.Errorf("the workspace came back %q, so the browser lands nowhere", choice.WorkspaceSlug)
	}
}

func TestAStashBelongingToAnotherWorkspaceIsNotHandedOver(t *testing.T) {
	held := appsFor(t, cloudConfig())

	held.states.EXPECT().
		Read(gomock.Any(), "the-handle").
		Return(entity.SCMAppState{
			Purpose:     entity.SCMAppChosen,
			Provider:    entity.SCMProviderGitHub,
			WorkspaceID: uuid.New(),
			Installations: []entity.SCMInstallation{
				{ExternalID: "884411", AccountLogin: "somebody-else"},
			},
		}, nil)

	_, err := held.service.Choice(context.Background(), held.workspace, "the-handle")

	if !errors.Is(err, entity.ErrSCMAppStateNotFound) {
		t.Fatalf(
			"a handle minted for another workspace was accepted (%v). An administrator of one "+
				"workspace would see, and could connect, another workspace's installations",
			err,
		)
	}
}

func registerInput(workspaceID uuid.UUID) service.RegisterSCMAppInput {
	return service.RegisterSCMAppInput{
		WorkspaceID:  workspaceID,
		InstanceURL:  "https://norn.example",
		InstanceName: "Norn",
		HookURL:      "https://norn.example/v1/source-control/github-app",
		RedirectURL:  "https://norn.example/v1/source-control/github-app/registered",
		CallbackURL:  "https://norn.example/v1/source-control/github-app/connected",
	}
}

func TestTheApplicationScreenNamesNoInstallationAtAll(t *testing.T) {
	held := appsFor(t, cloudConfig())

	stored := entity.SCMApp{
		ID:            uuid.New(),
		Provider:      entity.SCMProviderGitHub,
		BaseURL:       "https://api.github.com",
		Slug:          "nornbot",
		ExternalAppID: "4711",
	}

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, "https://api.github.com").
		Return(stored, nil)

	found, err := held.service.Application(
		context.Background(), held.workspace, entity.SCMProviderGitHub,
	)
	if err != nil {
		t.Fatalf("Application: %v", err)
	}

	if found.ID != stored.ID {
		t.Fatalf("the screen was handed application %s, want %s", found.ID, stored.ID)
	}
}

func TestASecondApplicationIsRefusedOnceOneIsRegistered(t *testing.T) {
	held := appsFor(t, config.SourceControl{})
	stored := entity.SCMApp{ID: uuid.New(), ExternalAppID: "4711"}

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, "https://api.github.com").
		Return(stored, nil)

	held.connections.EXPECT().CountByApp(gomock.Any(), stored.ID).Return(2, nil)

	_, err := held.service.Registration(context.Background(), registerInput(held.workspace))

	if !errors.Is(err, entity.ErrSCMAppExists) {
		t.Fatalf(
			"registering over the instance's application returned %v. One application serves "+
				"every workspace, so a second registration replaces the private key and the "+
				"webhook secret the others depend on, and hands whoever registered a secret "+
				"that verifies deliveries into any of them",
			err,
		)
	}
}

func TestARegistrationCallbackIsRefusedOnceOneIsRegistered(t *testing.T) {
	held := appsFor(t, config.SourceControl{})

	held.states.EXPECT().
		Take(gomock.Any(), "the-state").
		Return(entity.SCMAppState{
			Purpose:     entity.SCMAppRegister,
			Provider:    entity.SCMProviderGitHub,
			WorkspaceID: held.workspace,
		}, nil)

	stored := entity.SCMApp{ID: uuid.New(), ExternalAppID: "4711"}

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, "https://api.github.com").
		Return(stored, nil)

	held.connections.EXPECT().CountByApp(gomock.Any(), stored.ID).Return(1, nil)

	_, err := held.service.CompleteRegistration(context.Background(), "the-code", "the-state")

	if !errors.Is(err, entity.ErrSCMAppExists) {
		t.Fatalf(
			"the callback adopted a second application (%v). It carries no session, so a state "+
				"minted while the instance held none can be spent after one is registered, and "+
				"the check where the exchange begins never runs again",
			err,
		)
	}
}

func TestAnApplicationNobodyIsConnectedThroughCanBeRegisteredAgain(t *testing.T) {
	held := appsFor(t, config.SourceControl{})
	stored := entity.SCMApp{ID: uuid.New(), ExternalAppID: "4711"}

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, "https://api.github.com").
		Return(stored, nil)

	held.connections.EXPECT().CountByApp(gomock.Any(), stored.ID).Return(0, nil)

	held.forgeApp.EXPECT().
		ManifestTarget(gomock.Any(), gomock.Any()).
		Return("https://github.com/settings/apps/new")

	held.states.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	if _, err := held.service.Registration(
		context.Background(), registerInput(held.workspace),
	); err != nil {
		t.Fatalf(
			"registering over an application nothing is connected through returned %v. A "+
				"registration that went wrong halfway leaves a row nobody uses, and refusing "+
				"that one leaves the instance with no way back",
			err,
		)
	}
}

func TestAStashIsNotSpendableByAnotherAdministrator(t *testing.T) {
	held := appsFor(t, cloudConfig())

	held.states.EXPECT().
		Read(gomock.Any(), "the-handle").
		Return(entity.SCMAppState{
			Purpose:     entity.SCMAppChosen,
			Provider:    entity.SCMProviderGitHub,
			WorkspaceID: held.workspace,
			AccountID:   uuid.New(),
			Installations: []entity.SCMInstallation{
				{ExternalID: "884411", AccountLogin: "somebody-elses-org"},
			},
		}, nil)

	_, err := held.service.Choice(context.Background(), held.workspace, "the-handle")

	if !errors.Is(err, entity.ErrSCMAppStateNotFound) {
		t.Fatalf(
			"a stash minted by another administrator was read back (%v). It lists the forge "+
				"installations one person reached with their own account, so sharing a "+
				"workspace with them is not a claim on it",
			err,
		)
	}
}

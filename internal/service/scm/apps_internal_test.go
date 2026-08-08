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
	service    *apps
	registry   *scmrepo.MockSCMApp
	states     *statemock.MockSCMAppState
	forgeApp   *MockForgeApp
	authorizer *authorizermock.MockAuthorizer
	workspace  uuid.UUID
}

func appsFor(t *testing.T, cfg config.SourceControl) appHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	registry := scmrepo.NewMockSCMApp(ctrl)
	states := statemock.NewMockSCMAppState(ctrl)
	forgeApp := NewMockForgeApp(ctrl)
	forges := NewMockForges(ctrl)
	hub := NewMockForge(ctrl)
	authorizer := authorizermock.NewMockAuthorizer(ctrl)

	forges.EXPECT().App(entity.SCMProviderGitHub).Return(forgeApp, nil).AnyTimes()
	forges.EXPECT().Lookup(entity.SCMProviderGitHub).Return(hub, nil).AnyTimes()
	hub.EXPECT().Endpoint().Return("https://api.github.com").AnyTimes()

	workspace := uuid.New()

	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Role:      entity.MembershipRoleAdmin,
			Actor:     entity.Actor{AccountID: uuid.New()},
			Workspace: entity.Workspace{ID: workspace, Slug: "northwind"},
		}, nil).
		AnyTimes()

	held := NewApps(registry, states, forges, authorizer, cfg)

	return appHarness{
		service:    held.(*apps),
		registry:   registry,
		states:     states,
		forgeApp:   forgeApp,
		authorizer: authorizer,
		workspace:  workspace,
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

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, gomock.Any()).
		Return(registered, nil)

	held.forgeApp.EXPECT().
		ExchangeCode(gomock.Any(), registered, "the-code", gomock.Any()).
		Return("gho-the-user-token", nil)

	held.forgeApp.EXPECT().
		Installations(gomock.Any(), registered, "gho-the-user-token").
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
		Take(gomock.Any(), "the-handle").
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

package scm

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
)

type identifying struct {
	resolver *credentials
	registry *scmrepo.MockSCMApp
	forge    *MockForge
	forgeApp *MockForgeApp
}

func identifyingFor(t *testing.T) identifying {
	t.Helper()

	ctrl := gomock.NewController(t)

	registry := scmrepo.NewMockSCMApp(ctrl)
	hub := NewMockForge(ctrl)
	forgeApp := NewMockForgeApp(ctrl)
	forges := NewMockForges(ctrl)

	forges.EXPECT().Lookup(entity.SCMProviderGitHub).Return(hub, nil).AnyTimes()
	forges.EXPECT().App(entity.SCMProviderGitHub).Return(forgeApp, nil).AnyTimes()
	hub.EXPECT().Endpoint().Return("https://api.github.com").AnyTimes()

	return identifying{
		resolver: newCredentials(
			scmrepo.NewMockSCMConnection(ctrl),
			registry,
			forges,
			forge.NewCredentials(),
		),
		registry: registry,
		forge:    hub,
		forgeApp: forgeApp,
	}
}

func githubTarget(token string) entity.SCMTarget {
	return entity.SCMTarget{Provider: entity.SCMProviderGitHub, Token: token}
}

func TestAnInstallationIsNeverAskedWhoItsUserIs(t *testing.T) {
	held := identifyingFor(t)
	registered := entity.SCMApp{ID: uuid.New(), Slug: "norn-northwind"}

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, "https://api.github.com").
		Return(registered, nil)

	held.forgeApp.EXPECT().
		InstallationRepositories(gomock.Any(), registered, "ghs-installation").
		Return([]entity.SCMRemoteRepository{{FullName: "flagroll/platform"}}, nil)

	login, err := held.resolver.identify(
		context.Background(),
		githubTarget("ghs-installation"),
		entity.SCMAuthApp,
		"flagroll",
	)
	if err != nil {
		t.Fatalf(
			"identifying an installation failed: %v. An installation token cannot read a user, "+
				"so asking who it is refuses every connection made through an application",
			err,
		)
	}

	if login != "norn-northwind[bot]" {
		t.Fatalf(
			"an installation identified as %q. It acts as the application, and that is the name "+
				"that appears against everything it does on the forge",
			login,
		)
	}
}

func TestAnInstallationWhoseApplicationHasNoSlugFallsBackToTheAccount(t *testing.T) {
	held := identifyingFor(t)
	registered := entity.SCMApp{ID: uuid.New()}

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, gomock.Any()).
		Return(registered, nil)

	held.forgeApp.EXPECT().
		InstallationRepositories(gomock.Any(), registered, gomock.Any()).
		Return(nil, nil)

	login, err := held.resolver.identify(
		context.Background(),
		githubTarget("ghs-installation"),
		entity.SCMAuthApp,
		"flagroll",
	)
	if err != nil {
		t.Fatalf("identify: %v", err)
	}

	if login != "flagroll" {
		t.Fatalf("identified as %q, want the account the installation belongs to", login)
	}
}

func TestAnInstallationThatCannotReachTheForgeIsNotCalledHealthy(t *testing.T) {
	held := identifyingFor(t)
	registered := entity.SCMApp{ID: uuid.New(), Slug: "norn-northwind"}

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, gomock.Any()).
		Return(registered, nil)

	held.forgeApp.EXPECT().
		InstallationRepositories(gomock.Any(), registered, gomock.Any()).
		Return(nil, entity.ErrSCMInstallationNotFound)

	if _, err := held.resolver.identify(
		context.Background(),
		githubTarget("ghs-installation"),
		entity.SCMAuthApp,
		"flagroll",
	); !errors.Is(err, entity.ErrSCMInstallationNotFound) {
		t.Fatalf(
			"identifying returned %v. Naming the application without asking the forge anything "+
				"would report a revoked installation as working on every sweep",
			err,
		)
	}
}

func TestAPastedTokenIsStillIdentifiedByAskingWhoItIs(t *testing.T) {
	held := identifyingFor(t)

	held.forge.EXPECT().
		Identity(gomock.Any(), githubTarget("ghp-personal")).
		Return("rae", nil)

	login, err := held.resolver.identify(
		context.Background(),
		githubTarget("ghp-personal"),
		entity.SCMAuthToken,
		"",
	)
	if err != nil {
		t.Fatalf("identify: %v", err)
	}

	if login != "rae" {
		t.Fatalf("a pasted token identified as %q, want the account it belongs to", login)
	}
}

func TestAnApplicationIsLookedUpUnderTheEndpointItWasStoredAgainst(t *testing.T) {
	held := identifyingFor(t)

	held.registry.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, "https://api.github.com").
		Return(entity.SCMApp{ID: uuid.New()}, nil)

	if _, err := held.resolver.application(
		context.Background(), entity.SCMProviderGitHub, "",
	); err != nil {
		t.Fatalf(
			"looking up the application for the public forge failed: %v. A connection to "+
				"github.com carries no address of its own, but the application is stored against "+
				"the endpoint, so an empty address has to resolve to it",
			err,
		)
	}
}

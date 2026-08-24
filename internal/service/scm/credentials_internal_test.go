package scm

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
)

type held struct {
	resolver    *credentials
	connections *scmrepo.MockSCMConnection
	apps        *scmrepo.MockSCMApp
	forgeApp    *MockForgeApp
	cache       *forge.Credentials
}

func resolver(t *testing.T) held {
	t.Helper()

	ctrl := gomock.NewController(t)

	connections := scmrepo.NewMockSCMConnection(ctrl)
	apps := scmrepo.NewMockSCMApp(ctrl)
	forgeApp := NewMockForgeApp(ctrl)
	forges := NewMockForges(ctrl)
	cache := forge.NewCredentials()

	forges.EXPECT().
		App(entity.SCMProviderGitHub).
		Return(forgeApp, nil).
		AnyTimes()

	return held{
		resolver:    newCredentials(connections, apps, forges, cache),
		connections: connections,
		apps:        apps,
		forgeApp:    forgeApp,
		cache:       cache,
	}
}

func installed(appID uuid.UUID) entity.SCMConnection {
	return entity.SCMConnection{
		ID:             uuid.New(),
		Provider:       entity.SCMProviderGitHub,
		AuthKind:       entity.SCMAuthApp,
		AppID:          appID,
		InstallationID: "884411",
	}
}

func TestATokenConnectionStillReadsItsOwnSealedSecret(t *testing.T) {
	on := resolver(t)

	connection := entity.SCMConnection{
		ID:       uuid.New(),
		Provider: entity.SCMProviderGitLab,
		AuthKind: entity.SCMAuthToken,
	}

	on.connections.EXPECT().Token(gomock.Any(), connection.ID).Return("glpat-secret", nil)

	token, err := on.resolver.token(context.Background(), connection)
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	if token != "glpat-secret" {
		t.Fatalf(
			"a token connection resolved to %q; GitLab and Gitea have no application to install, "+
				"so the pasted secret is the only credential they will ever have",
			token,
		)
	}
}

func TestAnInstallationIsMintedOnceAndReusedUntilItRunsOut(t *testing.T) {
	on := resolver(t)
	appID := uuid.New()
	connection := installed(appID)

	on.apps.EXPECT().
		Secrets(gomock.Any(), appID).
		Return(entity.SCMApp{ID: appID, ExternalAppID: "17"}, nil).
		Times(1)

	on.forgeApp.EXPECT().
		MintInstallationToken(gomock.Any(), gomock.Any(), "884411").
		Return(entity.SCMCredential{
			Token:     "ghs-first",
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil).
		Times(1)

	for attempt := range 3 {
		token, err := on.resolver.token(context.Background(), connection)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}

		if token != "ghs-first" {
			t.Fatalf("attempt %d resolved to %q, want the minted token", attempt, token)
		}
	}
}

func TestAnInstallationTokenThatHasRunOutIsBakedAgain(t *testing.T) {
	on := resolver(t)
	appID := uuid.New()
	connection := installed(appID)

	on.apps.EXPECT().
		Secrets(gomock.Any(), appID).
		Return(entity.SCMApp{ID: appID, ExternalAppID: "17"}, nil).
		Times(2)

	first := on.forgeApp.EXPECT().
		MintInstallationToken(gomock.Any(), gomock.Any(), "884411").
		Return(entity.SCMCredential{
			Token:     "ghs-stale",
			ExpiresAt: time.Now().Add(-time.Minute),
		}, nil)

	on.forgeApp.EXPECT().
		MintInstallationToken(gomock.Any(), gomock.Any(), "884411").
		Return(entity.SCMCredential{
			Token:     "ghs-fresh",
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil).
		After(first)

	if _, err := on.resolver.token(context.Background(), connection); err != nil {
		t.Fatalf("the first mint: %v", err)
	}

	token, err := on.resolver.token(context.Background(), connection)
	if err != nil {
		t.Fatalf("the second mint: %v", err)
	}

	if token != "ghs-fresh" {
		t.Fatalf(
			"a token that had already run out was handed out again as %q. An installation token "+
				"lives an hour, so every sync after that hour would fail with nothing to retry",
			token,
		)
	}
}

func TestATokenAboutToRunOutIsNotHandedToACallThatOutlivesIt(t *testing.T) {
	cache := forge.NewCredentials()
	key := uuid.New()
	now := time.Now()

	cache.Put(key, entity.SCMCredential{Token: "ghs-expiring", ExpiresAt: now.Add(time.Minute)})

	if _, found := cache.Get(key, now); found {
		t.Fatal(
			"a token a minute from running out was handed out. A sync started with it can still " +
				"be running when it dies, so a margin is what keeps a long read from failing halfway",
		)
	}
}

func TestAnInstallationThatWasNeverChosenIsRefusedRatherThanReadAsEmpty(t *testing.T) {
	on := resolver(t)

	connection := entity.SCMConnection{
		ID:       uuid.New(),
		Provider: entity.SCMProviderGitHub,
		AuthKind: entity.SCMAuthApp,
		AppID:    uuid.New(),
	}

	if _, err := on.resolver.token(context.Background(), connection); err == nil {
		t.Fatal(
			"a connection with no installation resolved to a credential. An empty installation id " +
				"would be pasted straight into the mint path and ask the forge for nobody's token",
		)
	}
}

func TestListingWhatAnInstallationReachesMintsAFreshToken(t *testing.T) {
	on := resolver(t)
	appID := uuid.New()
	connection := installed(appID)

	on.apps.EXPECT().
		Secrets(gomock.Any(), appID).
		Return(entity.SCMApp{ID: appID, ExternalAppID: "17"}, nil).
		Times(2)

	first := on.forgeApp.EXPECT().
		MintInstallationToken(gomock.Any(), gomock.Any(), "884411").
		Return(entity.SCMCredential{
			Token:     "ghs-before-the-grant",
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil)

	on.forgeApp.EXPECT().
		MintInstallationToken(gomock.Any(), gomock.Any(), "884411").
		Return(entity.SCMCredential{
			Token:     "ghs-after-the-grant",
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil).
		After(first)

	if _, err := on.resolver.token(context.Background(), connection); err != nil {
		t.Fatalf("the first mint: %v", err)
	}

	token, err := on.resolver.refresh(context.Background(), connection)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if token != "ghs-after-the-grant" {
		t.Fatalf(
			"listing what the installation reaches reused %q. An installation token carries the "+
				"repositories the installation reached when it was minted, so a repository "+
				"granted since is invisible to it for the rest of its hour — which is exactly "+
				"when somebody comes to connect the one they have just granted",
			token,
		)
	}

	if kept, found := on.cache.Get(connection.ID, time.Now()); !found || kept != token {
		t.Errorf(
			"the fresh token was not kept, so the connect that follows this listing mints again",
		)
	}
}

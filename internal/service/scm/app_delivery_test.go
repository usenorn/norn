package scm_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

type appDelivery struct {
	harness    *advanceHarness
	appID      uuid.UUID
	connection entity.SCMConnection
	repository entity.SCMRepository
	body       []byte
}

func appDeliveryFor(t *testing.T) appDelivery {
	t.Helper()

	held := appDelivery{
		harness: newAdvanceHarness(t),
		appID:   uuid.New(),
		body:    []byte(`{"installation":{"id":884411},"repository":{"full_name":"flagroll/platform"}}`),
	}

	held.connection = entity.SCMConnection{
		ID:             uuid.New(),
		WorkspaceID:    uuid.New(),
		Provider:       entity.SCMProviderGitHub,
		AuthKind:       entity.SCMAuthApp,
		AppID:          held.appID,
		InstallationID: "884411",
	}

	held.repository = entity.SCMRepository{
		ID:           uuid.New(),
		WorkspaceID:  held.connection.WorkspaceID,
		ConnectionID: held.connection.ID,
		Provider:     entity.SCMProviderGitHub,
		FullName:     "flagroll/platform",
	}

	h := held.harness

	h.forges.EXPECT().App(entity.SCMProviderGitHub).Return(h.forgeApp, nil).AnyTimes()
	h.forges.EXPECT().Lookup(entity.SCMProviderGitHub).Return(h.forge, nil).AnyTimes()
	h.forge.EXPECT().Endpoint().Return("https://api.github.com").AnyTimes()

	h.forgeApp.EXPECT().
		Route(gomock.Any()).
		Return(entity.SCMDeliveryRoute{
			InstallationID: "884411",
			FullName:       "flagroll/platform",
		}, nil).
		AnyTimes()

	h.apps.EXPECT().
		Get(gomock.Any(), entity.SCMProviderGitHub, "https://api.github.com").
		Return(entity.SCMApp{ID: held.appID, Provider: entity.SCMProviderGitHub}, nil).
		AnyTimes()

	h.apps.EXPECT().
		Secrets(gomock.Any(), held.appID).
		Return(entity.SCMApp{ID: held.appID, WebhookSecret: "the-app-secret"}, nil).
		AnyTimes()

	return held
}

func TestADeliveryToTheApplicationIsVerifiedWithTheApplicationSecret(t *testing.T) {
	held := appDeliveryFor(t)
	h := held.harness

	h.forge.EXPECT().
		Verify("the-app-secret", gomock.Any(), held.body).
		Return(entity.SCMDelivery{ExternalID: "delivery-1"}, nil)

	h.connections.EXPECT().
		GetByInstallation(gomock.Any(), held.appID, "884411").
		Return(held.connection, nil)

	h.repositories.EXPECT().
		GetByFullName(gomock.Any(), held.connection.ID, "flagroll/platform").
		Return(held.repository, nil)

	var recorded entity.SCMDelivery

	h.deliveries.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, delivery entity.SCMDelivery) (uuid.UUID, error) {
			recorded = delivery

			return uuid.New(), nil
		})

	h.repositories.EXPECT().RecordSeen(gomock.Any(), held.repository.ID, gomock.Any()).Return(nil)
	h.jobs.EXPECT().EnqueueSCMDelivery(gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.sync.AcceptFromApp(
		context.Background(), entity.SCMProviderGitHub, http.Header{}, held.body,
	); err != nil {
		t.Fatalf("AcceptFromApp: %v", err)
	}

	if recorded.RepositoryID != held.repository.ID {
		t.Fatalf(
			"the delivery was held against repository %s, want %s. One address serves every "+
				"repository the application is installed on, so the body is the only thing that "+
				"says which one it belongs to",
			recorded.RepositoryID, held.repository.ID,
		)
	}

	if recorded.WorkspaceID != held.repository.WorkspaceID {
		t.Errorf("the delivery landed in workspace %s, want %s",
			recorded.WorkspaceID, held.repository.WorkspaceID)
	}
}

func TestADeliverySignedWithSomebodyElsesSecretIsRefused(t *testing.T) {
	held := appDeliveryFor(t)
	h := held.harness

	h.forge.EXPECT().
		Verify("the-app-secret", gomock.Any(), held.body).
		Return(entity.SCMDelivery{}, entity.ErrSCMSignatureInvalid)

	_, err := h.sync.AcceptFromApp(
		context.Background(), entity.SCMProviderGitHub, http.Header{}, held.body,
	)

	if !errors.Is(err, entity.ErrSCMSignatureInvalid) {
		t.Fatalf(
			"a delivery that did not verify returned %v. Anybody who learns the address could "+
				"post whatever they liked and it would be applied to a workspace's issues",
			err,
		)
	}
}

func TestADeliveryNamingARepositoryThisInstallationDoesNotHoldIsRefused(t *testing.T) {
	held := appDeliveryFor(t)
	h := held.harness

	h.forge.EXPECT().
		Verify(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.SCMDelivery{ExternalID: "delivery-1"}, nil)

	h.connections.EXPECT().
		GetByInstallation(gomock.Any(), held.appID, "884411").
		Return(held.connection, nil)

	h.repositories.EXPECT().
		GetByFullName(gomock.Any(), held.connection.ID, "flagroll/platform").
		Return(entity.SCMRepository{}, entity.ErrSCMRepositoryNotFound)

	_, err := h.sync.AcceptFromApp(
		context.Background(), entity.SCMProviderGitHub, http.Header{}, held.body,
	)

	if !errors.Is(err, entity.ErrSCMRepositoryNotFound) {
		t.Fatalf(
			"a delivery for a repository nobody connected returned %v, want it named as an "+
				"unconnected repository. The application is installed on repositories a workspace "+
				"never chose and each of those still posts here, so this is the ordinary case and "+
				"calling it a signature failure hides the deliveries that really did not verify",
			err,
		)
	}

	if errors.Is(err, entity.ErrSCMSignatureInvalid) {
		t.Fatal("an unconnected repository was reported as forgery")
	}
}

func TestADeliveryFromAnInstallationNobodyConnectedIsRefused(t *testing.T) {
	held := appDeliveryFor(t)
	h := held.harness

	h.forge.EXPECT().
		Verify(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.SCMDelivery{ExternalID: "delivery-1"}, nil)

	h.connections.EXPECT().
		GetByInstallation(gomock.Any(), held.appID, "884411").
		Return(entity.SCMConnection{}, entity.ErrSCMConnectionNotFound)

	_, err := h.sync.AcceptFromApp(
		context.Background(), entity.SCMProviderGitHub, http.Header{}, held.body,
	)

	if !errors.Is(err, entity.ErrSCMConnectionNotFound) {
		t.Fatalf(
			"a delivery from an unconnected installation returned %v, want it named as an "+
				"unknown installation rather than as a bad signature",
			err,
		)
	}

	if errors.Is(err, entity.ErrSCMSignatureInvalid) {
		t.Fatal("an unconnected installation was reported as forgery")
	}
}

func TestARepositoryTheApplicationDeliversForRefusesItsOwnAddress(t *testing.T) {
	held := appDeliveryFor(t)
	h := held.harness

	h.repositories.EXPECT().
		GetForDelivery(gomock.Any(), held.repository.ID).
		Return(held.repository, nil)

	h.repositories.EXPECT().
		WebhookSecret(gomock.Any(), held.repository.ID).
		Return("", nil)

	_, err := h.sync.Accept(
		context.Background(),
		held.repository.ID,
		entity.SCMProviderGitHub,
		http.Header{},
		held.body,
	)

	if !errors.Is(err, entity.ErrSCMSignatureInvalid) {
		t.Fatalf(
			"a delivery to a repository holding no secret came back %v, want a refusal. Verifying "+
				"against an empty secret signs the body under a key anybody has, so whoever knows "+
				"the repository's id could have this instance act on a payload they wrote",
			err,
		)
	}
}

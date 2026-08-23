package previewgateway_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAGatewaySecretIsAnsweredOnceAndTradedForSomethingShortLived(t *testing.T) {
	h := newHarness(t)

	secret := h.enrolled(t, "edge-eu")

	access, err := h.service.Exchange(context.Background(), secret)
	if err != nil {
		t.Fatalf("exchange the secret: %v", err)
	}

	if access.Token == secret {
		t.Fatal(
			"the long-lived secret was handed back as the working credential. It would then " +
				"travel on every introspection call rather than sitting in the gateway's config",
		)
	}

	if !entity.LooksLikePreviewGatewayToken(access.Token) {
		t.Fatalf("the access token %q is not one this server would recognise", access.Token)
	}

	gateway, err := h.service.Authenticate(context.Background(), access.Token)
	if err != nil {
		t.Fatalf("authenticate with the access token: %v", err)
	}

	if gateway.Name != "edge-eu" {
		t.Fatalf("the token authenticated as %q, want edge-eu", gateway.Name)
	}
}

func TestNothingButAGatewayCredentialGetsThroughTheGatewayDoor(t *testing.T) {
	h := newHarness(t)
	secret := h.enrolled(t, "edge-eu")

	for name, presented := range map[string]string{
		"nothing at all":            "",
		"the enrolment secret":      secret,
		"a made-up access token":    entity.PreviewGatewayAccessPrefix + "nonsense",
		"a runner's own credential": entity.RunnerAccessPrefix + "nonsense",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := h.service.Authenticate(context.Background(), presented); err == nil {
				t.Fatalf(
					"%q was accepted as a gateway. Whatever holds this credential can ask about "+
						"every preview on the instance",
					name,
				)
			}
		})
	}
}

func TestRevokingAGatewayShutsOutTheCredentialItIsAlreadyHolding(t *testing.T) {
	h := newHarness(t)
	secret := h.enrolled(t, "edge-eu")

	access, err := h.service.Exchange(context.Background(), secret)
	if err != nil {
		t.Fatalf("exchange the secret: %v", err)
	}

	gateway, err := h.service.Authenticate(context.Background(), access.Token)
	if err != nil {
		t.Fatalf("authenticate before the revocation: %v", err)
	}

	if _, err := h.service.Revoke(context.Background(), gateway.ID); err != nil {
		t.Fatalf("revoke the gateway: %v", err)
	}

	if _, err := h.service.Authenticate(context.Background(), access.Token); err == nil {
		t.Fatal(
			"a revoked gateway is still holding a working credential. Revocation that only " +
				"stops the next exchange leaves it inside for the rest of its window",
		)
	}

	if _, err := h.service.Exchange(context.Background(), secret); err == nil {
		t.Fatal("a revoked gateway exchanged its secret for a fresh credential")
	}
}

func TestTwoGatewaysNeverAnswerToTheSameName(t *testing.T) {
	h := newHarness(t)
	h.enrolled(t, "edge-eu")

	_, _, err := h.service.Enrol(context.Background(), "edge-eu")

	refusedWith(t, err, entity.ErrPreviewGatewayNameTaken)
}

func TestAGatewayHasToBeNamedBeforeItIsEnrolled(t *testing.T) {
	h := newHarness(t)

	if _, _, err := h.service.Enrol(context.Background(), "   "); err == nil {
		t.Fatal(
			"a gateway was enrolled with no name. A list of unnamed credentials is one nobody " +
				"can safely revoke from",
		)
	}
}

func TestExchangingASecretRecordsThatTheGatewayCalled(t *testing.T) {
	h := newHarness(t)
	secret := h.enrolled(t, "edge-eu")

	if _, err := h.service.Exchange(context.Background(), secret); err != nil {
		t.Fatalf("exchange the secret: %v", err)
	}

	if len(h.seen) != 1 {
		t.Fatal(
			"nothing recorded that the gateway called. A credential nothing has used is what " +
				"an operator looks for when deciding which to withdraw",
		)
	}
}

func TestARevokedGatewayIsStillListedRatherThanQuietlyGone(t *testing.T) {
	h := newHarness(t)
	secret := h.enrolled(t, "edge-eu")

	access, err := h.service.Exchange(context.Background(), secret)
	if err != nil {
		t.Fatalf("exchange the secret: %v", err)
	}

	gateway, err := h.service.Authenticate(context.Background(), access.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	if _, err := h.service.Revoke(context.Background(), gateway.ID); err != nil {
		t.Fatalf("revoke the gateway: %v", err)
	}

	held, err := h.service.List(context.Background())
	if err != nil {
		t.Fatalf("list the gateways: %v", err)
	}

	if len(held) != 1 || held[0].Status != entity.PreviewGatewayRevoked {
		t.Fatalf(
			"a revoked gateway is no longer on the list. It has to stay visible, or the name " +
				"comes back and nobody knows it was ever withdrawn",
		)
	}
}

func TestRevokingAGatewayThatWasNeverEnrolledSaysSo(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.Revoke(context.Background(), uuid.New())

	refusedWith(t, err, entity.ErrPreviewGatewayNotFound)
}

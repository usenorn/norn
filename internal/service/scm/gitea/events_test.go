package gitea_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func signedWith(key, body []byte) http.Header {
	sum := hmac.New(sha256.New, key)
	sum.Write(body)

	header := http.Header{}
	header.Set("X-Gitea-Signature", hex.EncodeToString(sum.Sum(nil)))
	header.Set("X-Gitea-Event", "push")
	header.Set("X-Gitea-Delivery", "d-1")

	return header
}

func TestADeliveryIsAcceptedOnlyUnderTheSecretTheRepositoryHolds(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	held := "nrnscm_a-shared-secret"

	delivery, err := adapter(t).Verify(held, signedWith([]byte(held), body), body)
	if err != nil {
		t.Fatalf("a correctly signed delivery was refused: %v", err)
	}

	if delivery.ExternalID != "d-1" || delivery.Event != "push" {
		t.Fatalf("Verify = %+v, want the delivery id and event carried through", delivery)
	}

	if _, err := adapter(t).Verify(held, signedWith([]byte("another-secret"), body), body); !errors.Is(
		err, entity.ErrSCMSignatureInvalid,
	) {
		t.Fatalf("a delivery signed under a different secret came back %v, want a refusal", err)
	}
}

func TestADeliveryIsRefusedWhenTheRepositoryHoldsNoSecretOfItsOwn(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)

	if _, err := adapter(t).Verify("", signedWith(nil, body), body); !errors.Is(
		err, entity.ErrSCMSignatureInvalid,
	) {
		t.Fatalf(
			"a delivery signed under an empty key was accepted (%v). An empty key is one anybody "+
				"has, so the signature would prove nothing at all",
			err,
		)
	}
}

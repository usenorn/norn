package crypter_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/crypter"
)

func withKey(t *testing.T) *crypter.Crypter {
	t.Helper()

	key := make([]byte, crypter.KeyBytes)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	sealed, err := crypter.New(config.Security{EncryptionKey: base64.StdEncoding.EncodeToString(key)})
	if err != nil {
		t.Fatalf("build crypter: %v", err)
	}

	return sealed
}

func TestASecretSurvivesTheRoundTrip(t *testing.T) {
	c := withKey(t)
	secret := []byte("s3cr3t-client-value-from-the-provider")

	sealed, err := c.Seal(secret)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if bytes.Contains(sealed, secret) {
		t.Fatal(
			"the ciphertext contains the plaintext. Anyone reading the database row would have " +
				"working provider credentials, which is the whole point of sealing it.",
		)
	}

	opened, err := c.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if !bytes.Equal(opened, secret) {
		t.Fatalf("opened %q, want %q", opened, secret)
	}
}

func TestSealingTheSameSecretTwiceProducesDifferentCiphertext(t *testing.T) {
	c := withKey(t)
	secret := []byte("the same secret both times")

	first, err := c.Seal(secret)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	second, err := c.Seal(secret)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal(
			"two sealings matched byte for byte, so the nonce is not random. Equal ciphertexts " +
				"tell an observer that two workspaces share a provider secret.",
		)
	}
}

func TestATamperedCiphertextIsRefusedRatherThanDecrypted(t *testing.T) {
	c := withKey(t)

	sealed, err := c.Seal([]byte("client secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	sealed[len(sealed)-1] ^= 0xff

	if _, err := c.Open(sealed); !errors.Is(err, crypter.ErrCiphertext) {
		t.Fatalf("opening tampered ciphertext gave %v, want a refusal", err)
	}
}

func TestCiphertextShorterThanANonceIsRefused(t *testing.T) {
	c := withKey(t)

	if _, err := c.Open([]byte{1, 2, 3}); !errors.Is(err, crypter.ErrCiphertext) {
		t.Fatalf("opening a truncated value gave %v, want a refusal", err)
	}
}

func TestAnInstanceWithoutAKeyStillStartsButRefusesToSeal(t *testing.T) {
	c, err := crypter.New(config.Security{})
	if err != nil {
		t.Fatalf(
			"building a crypter without a key failed with %v. An install that never turns on "+
				"single sign-on must boot exactly as before.",
			err,
		)
	}

	if c.Ready() {
		t.Fatal("a keyless crypter reports itself ready")
	}

	if _, err := c.Seal([]byte("anything")); !errors.Is(err, crypter.ErrKeyMissing) {
		t.Fatalf("sealing without a key gave %v, want ErrKeyMissing", err)
	}
}

func TestAKeyThatIsPresentButWrongFailsAtStartup(t *testing.T) {
	for name, key := range map[string]string{
		"not base64": "!!!!not-base64!!!!",
		"too short":  base64.StdEncoding.EncodeToString([]byte("short")),
		"too long":   base64.StdEncoding.EncodeToString(make([]byte, crypter.KeyBytes+1)),
	} {
		if _, err := crypter.New(config.Security{EncryptionKey: key}); !errors.Is(err, crypter.ErrKeyMalformed) {
			t.Errorf(
				"%s: New gave %v, want ErrKeyMalformed. A key that was set but is unusable is a "+
					"misconfiguration and must stop the process, not surface later as a failed save.",
				name, err,
			)
		}
	}
}

func TestAnEmptyKeyIsAbsentRatherThanMalformed(t *testing.T) {
	c, err := crypter.New(config.Security{EncryptionKey: ""})
	if err != nil {
		t.Fatalf("an unset key was treated as malformed: %v", err)
	}

	if c.Ready() {
		t.Fatal("an unset key produced a ready crypter")
	}
}

func TestAKeySealedByOneInstanceCannotBeOpenedByAnother(t *testing.T) {
	first, second := withKey(t), withKey(t)

	sealed, err := first.Seal([]byte("client secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if _, err := second.Open(sealed); !errors.Is(err, crypter.ErrCiphertext) {
		t.Fatalf("a different key opened the ciphertext (%v)", err)
	}
}

func TestTheRefusalWithoutAKeyNamesTheSettingToChange(t *testing.T) {
	c, err := crypter.New(config.Security{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Seal([]byte("client secret"))
	if err == nil {
		t.Fatal("sealing without a key succeeded")
	}

	if !strings.Contains(err.Error(), "encryption key") {
		t.Fatalf(
			"the refusal reads %q, which does not tell an operator that a key is what is "+
				"missing. This is the one error an install without a key will ever see.",
			err,
		)
	}
}

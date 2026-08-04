package samlkey_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/pkg/samlkey"
)

func TestAGeneratedKeypairRoundTripsThroughStorage(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

	pair, err := samlkey.Generate("https://norn.example.com/sp", now)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	encodedKey := samlkey.MarshalPrivateKey(pair.PrivateKey)
	encodedCert := samlkey.MarshalCertificate(pair.Certificate)

	if !strings.Contains(string(encodedKey), "PRIVATE KEY") {
		t.Fatal("the encoded key is not PEM")
	}

	key, err := samlkey.ParsePrivateKey(encodedKey)
	if err != nil {
		t.Fatalf("ParsePrivateKey: %v", err)
	}

	if !key.Equal(pair.PrivateKey) {
		t.Fatal("the key that came back is not the one that went in")
	}

	certificate, err := samlkey.ParseCertificate(encodedCert)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}

	if certificate.Subject.CommonName != "https://norn.example.com/sp" {
		t.Fatalf("subject %q, want the entity ID", certificate.Subject.CommonName)
	}

	if !certificate.NotAfter.After(now.Add(5 * 365 * 24 * time.Hour)) {
		t.Fatalf(
			"the certificate expires %s, which is soon enough that a workspace would be forced "+
				"to re-register with its provider",
			certificate.NotAfter,
		)
	}
}

func TestACertificateIsReadableBothAsPEMAndAsBareBase64(t *testing.T) {
	pair, err := samlkey.Generate("https://norn.example.com/sp", time.Now())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	pemForm := samlkey.MarshalCertificate(pair.Certificate)

	bare := strings.Join(strings.FieldsFunc(pemForm, func(r rune) bool {
		return r == '\n' || r == '\r'
	})[1:], "")
	bare = strings.TrimSuffix(bare, "-----END CERTIFICATE-----")

	for name, encoded := range map[string]string{
		"PEM as written":          pemForm,
		"bare base64 as metadata": bare,
		"base64 with whitespace":  "  " + bare[:20] + "\n  " + bare[20:] + "\n",
	} {
		if _, err := samlkey.ParseCertificate(encoded); err != nil {
			t.Errorf(
				"%s was rejected (%v). Providers hand certificates over in all of these forms and "+
					"an administrator pasting one should not have to know which.",
				name, err,
			)
		}
	}
}

func TestRubbishIsRefusedRatherThanMisread(t *testing.T) {
	if _, err := samlkey.ParseCertificate("not a certificate at all"); !errors.Is(err, samlkey.ErrCertificateMalformed) {
		t.Errorf("ParseCertificate accepted rubbish: %v", err)
	}

	if _, err := samlkey.ParsePrivateKey([]byte("not a key")); !errors.Is(err, samlkey.ErrPrivateKeyMalformed) {
		t.Errorf("ParsePrivateKey accepted rubbish: %v", err)
	}
}

func TestTheEarliestExpiryWinsWhenAProviderPublishesSeveralCertificates(t *testing.T) {
	now := time.Now()

	soon, err := samlkey.Generate("soon", now.Add(-9*365*24*time.Hour))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	later, err := samlkey.Generate("later", now)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	expiry, err := samlkey.EarliestExpiry([]string{
		samlkey.MarshalCertificate(later.Certificate),
		samlkey.MarshalCertificate(soon.Certificate),
	})
	if err != nil {
		t.Fatalf("EarliestExpiry: %v", err)
	}

	if !expiry.Equal(soon.Certificate.NotAfter) {
		t.Fatal(
			"the later expiry was reported. A provider rotating certificates publishes both, and " +
				"warning on the later one means the warning arrives after sign-ins have started failing.",
		)
	}
}

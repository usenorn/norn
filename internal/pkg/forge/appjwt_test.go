package forge_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
)

func keyPair(t *testing.T, pkcs8 bool) (string, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate a key: %v", err)
	}

	if pkcs8 {
		encoded, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatalf("encode pkcs8: %v", err)
		}

		return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})), key
	}

	encoded := x509.MarshalPKCS1PrivateKey(key)

	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: encoded})), key
}

func verify(signed string, public *rsa.PublicKey) error {
	parts := strings.Split(signed, ".")
	if len(parts) != 3 {
		return errors.New("a signed token has three parts")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return err
	}

	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))

	return rsa.VerifyPKCS1v15(public, crypto.SHA256, digest[:], signature)
}

func TestBothKeyFormatsGitHubHandsOutAreAccepted(t *testing.T) {
	for _, pkcs8 := range []bool{false, true} {
		pemKey, _ := keyPair(t, pkcs8)

		if _, err := forge.ParseAppPrivateKey(pemKey); err != nil {
			t.Fatalf(
				"pkcs8=%t was refused: %v. GitHub hands out one format and a key converted by "+
					"openssl comes back as the other, so both arrive in practice",
				pkcs8, err,
			)
		}
	}
}

func TestSomethingThatIsNotAKeyIsRefusedByName(t *testing.T) {
	refused := []string{
		"",
		"not a key",
		"-----BEGIN RSA PRIVATE KEY-----\nnope\n-----END RSA PRIVATE KEY-----",
	}

	for _, value := range refused {
		if _, err := forge.ParseAppPrivateKey(value); !errors.Is(err, entity.ErrSCMPrivateKeyInvalid) {
			t.Errorf("ParseAppPrivateKey(%.24q) returned %v, want the private-key sentinel", value, err)
		}
	}
}

func TestTheApplicationTokenIsBackdatedAndShortLived(t *testing.T) {
	pemKey, key := keyPair(t, false)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

	signed, err := forge.AppJWT(pemKey, "12345", now)
	if err != nil {
		t.Fatalf("AppJWT: %v", err)
	}

	parts := strings.Split(signed, ".")

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode the claims: %v", err)
	}

	var claims struct {
		IssuedAt  int64  `json:"iat"`
		ExpiresAt int64  `json:"exp"`
		Issuer    string `json:"iss"`
	}

	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("read the claims: %v", err)
	}

	if claims.Issuer != "12345" {
		t.Errorf("iss = %q, want the application id", claims.Issuer)
	}

	if claims.IssuedAt >= now.Unix() {
		t.Error(
			"the token is not backdated. GitHub refuses one issued in its own future, and a " +
				"clock a few seconds fast is the ordinary case rather than the exception",
		)
	}

	if life := claims.ExpiresAt - claims.IssuedAt; life > 600 {
		t.Errorf("the token lives %ds; GitHub refuses anything over ten minutes", life)
	}

	if err := verify(signed, &key.PublicKey); err != nil {
		t.Fatalf("the signature does not verify against the key that signed it: %v", err)
	}
}

func TestAKeyThatDidNotSignItDoesNotVerify(t *testing.T) {
	pemKey, _ := keyPair(t, false)
	_, other := keyPair(t, false)

	signed, err := forge.AppJWT(pemKey, "12345", time.Now())
	if err != nil {
		t.Fatalf("AppJWT: %v", err)
	}

	if err := verify(signed, &other.PublicKey); err == nil {
		t.Fatal(
			"a token verified against a key that never signed it, so the signature proves nothing " +
				"and anybody could mint one",
		)
	}
}

func TestAnUnreadableKeyIsReportedRatherThanSigningNothing(t *testing.T) {
	if _, err := forge.AppJWT("rubbish", "12345", time.Now()); !errors.Is(err, entity.ErrSCMPrivateKeyInvalid) {
		t.Fatalf("AppJWT returned %v, want the private-key sentinel so a caller can say why", err)
	}
}

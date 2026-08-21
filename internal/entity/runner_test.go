package entity_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func assertion(runnerID uuid.UUID, issuedAt time.Time) entity.RunnerAssertion {
	return entity.RunnerAssertion{
		RunnerID: runnerID,
		Nonce:    "a-nonce-long-enough-to-pass",
		IssuedAt: issuedAt,
		Audience: entity.RunnerAssertionAudience,
	}
}

func TestEveryFieldOfAnAssertionIsCoveredByItsSignature(t *testing.T) {
	runnerID := uuid.New()
	issuedAt := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	base := assertion(runnerID, issuedAt)

	altered := map[string]entity.RunnerAssertion{
		"runner": {RunnerID: uuid.New(), Nonce: base.Nonce, IssuedAt: issuedAt, Audience: base.Audience},
		"nonce":  {RunnerID: runnerID, Nonce: "a-different-nonce-entirely", IssuedAt: issuedAt, Audience: base.Audience},
		"time":   {RunnerID: runnerID, Nonce: base.Nonce, IssuedAt: issuedAt.Add(time.Second), Audience: base.Audience},
		"audience": {
			RunnerID: runnerID, Nonce: base.Nonce, IssuedAt: issuedAt, Audience: "somewhere-else",
		},
	}

	for field, other := range altered {
		if string(other.SigningPayload()) == string(base.SigningPayload()) {
			t.Fatalf("changing the %s left the signed payload identical, so it is not bound by the signature", field)
		}
	}

	if !strings.HasPrefix(string(base.SigningPayload()), entity.RunnerAssertionVersion+"\n") {
		t.Fatalf("the payload does not lead with its version, so it can never be rotated safely")
	}
}

func TestAnAssertionVerifiesOnlyAgainstTheKeyThatSignedIt(t *testing.T) {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	other, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}

	signed := assertion(uuid.New(), time.Now().UTC())
	signed.Signature = ed25519.Sign(private, signed.SigningPayload())

	if err := signed.Verify(public); err != nil {
		t.Fatalf("verify with the signing key returned %v, want it accepted", err)
	}

	if err := signed.Verify(other); err != entity.ErrRunnerAssertionForged {
		t.Fatalf("verify with a stranger's key returned %v, want it refused as forged", err)
	}

	if err := signed.Verify(ed25519.PublicKey("short")); err != entity.ErrRunnerKeyMalformed {
		t.Fatalf("verify against a truncated key returned %v, want it refused as malformed", err)
	}
}

func TestAnAssertionIsFreshOnlyInsideTheSkewWindowInEitherDirection(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	skew := 3 * time.Minute

	cases := []struct {
		name  string
		drift time.Duration
		fresh bool
	}{
		{name: "exactly now", drift: 0, fresh: true},
		{name: "a little behind", drift: -2 * time.Minute, fresh: true},
		{name: "a little ahead", drift: 2 * time.Minute, fresh: true},
		{name: "on the edge behind", drift: -skew, fresh: true},
		{name: "on the edge ahead", drift: skew, fresh: true},
		{name: "too far behind", drift: -skew - time.Second, fresh: false},
		{name: "too far ahead", drift: skew + time.Second, fresh: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			held := assertion(uuid.New(), now.Add(testCase.drift))

			if held.Fresh(now, skew) != testCase.fresh {
				t.Fatalf("an assertion %s from now was judged fresh=%t, want %t",
					testCase.drift, !testCase.fresh, testCase.fresh)
			}
		})
	}
}

func TestARunnerStatusOnlyMovesBetweenDistinctKnownStates(t *testing.T) {
	cases := []struct {
		from    entity.RunnerStatus
		to      entity.RunnerStatus
		allowed bool
	}{
		{from: entity.RunnerStatusActive, to: entity.RunnerStatusRevoked, allowed: true},
		{from: entity.RunnerStatusRevoked, to: entity.RunnerStatusActive, allowed: true},
		{from: entity.RunnerStatusActive, to: entity.RunnerStatusActive, allowed: false},
		{from: entity.RunnerStatusActive, to: entity.RunnerStatus("retired"), allowed: false},
		{from: entity.RunnerStatus("retired"), to: entity.RunnerStatusActive, allowed: false},
	}

	for _, testCase := range cases {
		if testCase.from.CanTransitionTo(testCase.to) != testCase.allowed {
			t.Fatalf("%q to %q was judged %t, want %t",
				testCase.from, testCase.to, !testCase.allowed, testCase.allowed)
		}
	}
}

func TestOnlyAThirtyTwoByteKeyIsAcceptedAsADeviceKey(t *testing.T) {
	public, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	parsed, err := entity.ParseRunnerPublicKey(entity.EncodeRunnerPublicKey(public))
	if err != nil {
		t.Fatalf("parse a real key returned %v", err)
	}

	if !parsed.Equal(public) {
		t.Fatalf("the key did not survive the round trip")
	}

	for _, encoded := range []string{
		"",
		"not base64 at all!",
		base64.StdEncoding.EncodeToString([]byte("too short")),
		base64.StdEncoding.EncodeToString(append([]byte(public), 0)),
	} {
		if _, err := entity.ParseRunnerPublicKey(encoded); err != entity.ErrRunnerKeyMalformed {
			t.Fatalf("parsing %q returned %v, want it refused", encoded, err)
		}
	}
}

func TestARunnerSecretCarriesItsPrefixAndIsStoredOnlyAsAHash(t *testing.T) {
	secret, hash, err := entity.NewRunnerSecret(entity.RunnerAccessPrefix)
	if err != nil {
		t.Fatalf("new secret: %v", err)
	}

	if !entity.LooksLikeRunnerToken(secret) {
		t.Fatalf("secret %q lacks the prefix the bearer middleware dispatches on", secret)
	}

	if string(hash) == secret {
		t.Fatalf("the stored value is the secret itself")
	}

	if string(hash) != string(entity.HashRunnerSecret(secret)) {
		t.Fatalf("the returned hash is not the hash of the returned secret")
	}

	second, _, err := entity.NewRunnerSecret(entity.RunnerAccessPrefix)
	if err != nil {
		t.Fatalf("new secret: %v", err)
	}

	if second == secret {
		t.Fatalf("two secrets came back identical")
	}
}

func TestAnAssertionsNonceAndAudienceAreBounded(t *testing.T) {
	if field := entity.ValidateRunnerNonce("nonce", strings.Repeat("a", entity.RunnerNonceMinLen-1)); field.Code != entity.ValidationCodeTooShort {
		t.Fatalf("a short nonce was judged %q, want too_short", field.Code)
	}

	if field := entity.ValidateRunnerNonce("nonce", strings.Repeat("a", entity.RunnerNonceMaxLen+1)); field.Code != entity.ValidationCodeTooLong {
		t.Fatalf("a long nonce was judged %q, want too_long", field.Code)
	}

	if field := entity.ValidateRunnerNonce("nonce", strings.Repeat("a", entity.RunnerNonceMinLen)); field.Code != "" {
		t.Fatalf("a nonce at the minimum was judged %q, want it accepted", field.Code)
	}

	if field := entity.ValidateRunnerAudience("audience", "some-other-service"); field.Code != entity.ValidationCodeUnsupportedValue {
		t.Fatalf("an assertion for another audience was judged %q, want unsupported_value", field.Code)
	}

	if field := entity.ValidateRunnerAudience("audience", entity.RunnerAssertionAudience); field.Code != "" {
		t.Fatalf("the expected audience was judged %q, want it accepted", field.Code)
	}
}

func TestEveryPartOfTheHostIsRequired(t *testing.T) {
	complete := entity.RunnerHost{Hostname: "box", OS: "linux", Arch: "amd64", Version: "0.1.0"}

	if err := entity.NewValidationError(entity.ValidateRunnerHost(complete)...); err != nil {
		t.Fatalf("a complete host was refused: %v", err)
	}

	missing := []entity.RunnerHost{
		{OS: "linux", Arch: "amd64", Version: "0.1.0"},
		{Hostname: "box", Arch: "amd64", Version: "0.1.0"},
		{Hostname: "box", OS: "linux", Version: "0.1.0"},
		{Hostname: "box", OS: "linux", Arch: "amd64"},
	}

	for _, host := range missing {
		if err := entity.NewValidationError(entity.ValidateRunnerHost(host)...); err == nil {
			t.Fatalf("host %+v was accepted with a part missing", host)
		}
	}
}

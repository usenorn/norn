package entity_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestASignUpTokenIsStoredOnlyAsItsHash(t *testing.T) {
	token, hash, err := entity.NewSignUpToken()
	if err != nil {
		t.Fatalf("NewSignUpToken: %v", err)
	}

	if token == "" {
		t.Fatal("NewSignUpToken returned an empty token")
	}

	if !bytes.Equal(hash, entity.HashSignUpToken(token)) {
		t.Fatal("stored hash does not match the hash of the raw token")
	}

	if bytes.Contains(hash, []byte(token)) {
		t.Fatal("stored hash contains the raw token")
	}
}

func TestSignUpTokensAreUniquePerCall(t *testing.T) {
	first, _, err := entity.NewSignUpToken()
	if err != nil {
		t.Fatalf("NewSignUpToken: %v", err)
	}

	second, _, err := entity.NewSignUpToken()
	if err != nil {
		t.Fatalf("NewSignUpToken: %v", err)
	}

	if first == second {
		t.Fatal("two sign-up tokens were identical")
	}
}

func TestASignUpIsExpiredOnceItsDeadlinePasses(t *testing.T) {
	expires := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	signUp := entity.SignUp{ExpiresAt: expires}

	cases := []struct {
		name    string
		now     time.Time
		expired bool
	}{
		{"before", expires.Add(-time.Second), false},
		{"at the instant", expires, true},
		{"after", expires.Add(time.Second), true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := signUp.ExpiredAt(c.now); got != c.expired {
				t.Fatalf("ExpiredAt(%v) = %t, want %t", c.now, got, c.expired)
			}
		})
	}
}

func TestAConfirmedSignUpReportsItselfConfirmed(t *testing.T) {
	if (entity.SignUp{}).Confirmed() {
		t.Fatal("a sign-up with no confirmation time reported itself confirmed")
	}

	confirmedAt := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)

	if !(entity.SignUp{ConfirmedAt: &confirmedAt}).Confirmed() {
		t.Fatal("a confirmed sign-up did not report itself confirmed")
	}
}

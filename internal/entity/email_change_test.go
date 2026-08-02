package entity_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestNewEmailChangeTokenReturnsTheHashOfTheRawToken(t *testing.T) {
	token, hash, err := entity.NewEmailChangeToken()
	if err != nil {
		t.Fatalf("NewEmailChangeToken: %v", err)
	}

	if token == "" {
		t.Fatal("token is empty")
	}

	if !bytes.Equal(hash, entity.HashEmailChangeToken(token)) {
		t.Fatal("returned hash does not match the hash of the returned token")
	}
}

func TestEmailChangeTokensAreUniquePerCall(t *testing.T) {
	first, _, err := entity.NewEmailChangeToken()
	if err != nil {
		t.Fatalf("NewEmailChangeToken: %v", err)
	}

	second, _, err := entity.NewEmailChangeToken()
	if err != nil {
		t.Fatalf("NewEmailChangeToken: %v", err)
	}

	if first == second {
		t.Fatal("two generated tokens are identical")
	}
}

func TestEmailChangeExpiryIsInclusiveOfTheExpiryInstant(t *testing.T) {
	expires := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	change := entity.EmailChange{ExpiresAt: expires}

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
			if got := change.ExpiredAt(c.now); got != c.expired {
				t.Errorf("ExpiredAt = %t, want %t", got, c.expired)
			}
		})
	}
}

func TestEmailChangeReportsConfirmationState(t *testing.T) {
	pending := entity.EmailChange{}
	if pending.Confirmed() {
		t.Fatal("a change without a confirmation timestamp reports confirmed")
	}

	confirmedAt := time.Now()

	confirmed := entity.EmailChange{ConfirmedAt: &confirmedAt}
	if !confirmed.Confirmed() {
		t.Fatal("a change with a confirmation timestamp reports pending")
	}
}

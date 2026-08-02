package entity_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestAPasswordResetTokenIsStoredOnlyAsItsHash(t *testing.T) {
	token, hash, err := entity.NewPasswordResetToken()
	if err != nil {
		t.Fatalf("NewPasswordResetToken: %v", err)
	}

	if token == "" {
		t.Fatal("NewPasswordResetToken returned an empty token")
	}

	if !bytes.Equal(hash, entity.HashPasswordResetToken(token)) {
		t.Fatal("stored hash does not match the hash of the raw token")
	}

	if bytes.Contains(hash, []byte(token)) {
		t.Fatal("stored hash contains the raw token")
	}
}

func TestPasswordResetTokensAreUniquePerCall(t *testing.T) {
	first, _, err := entity.NewPasswordResetToken()
	if err != nil {
		t.Fatalf("NewPasswordResetToken: %v", err)
	}

	second, _, err := entity.NewPasswordResetToken()
	if err != nil {
		t.Fatalf("NewPasswordResetToken: %v", err)
	}

	if first == second {
		t.Fatal("two password reset tokens were identical")
	}
}

func TestAPasswordResetIsExpiredOnceItsDeadlinePasses(t *testing.T) {
	expires := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	reset := entity.PasswordReset{ExpiresAt: expires}

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
			if got := reset.ExpiredAt(c.now); got != c.expired {
				t.Fatalf("ExpiredAt(%s) = %v, want %v", c.now, got, c.expired)
			}
		})
	}
}

func TestAUsedPasswordResetReportsItselfUsed(t *testing.T) {
	usedAt := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)

	if (entity.PasswordReset{}).Used() {
		t.Fatal("a reset with no used_at reported itself used")
	}

	if !(entity.PasswordReset{UsedAt: &usedAt}).Used() {
		t.Fatal("a reset with a used_at did not report itself used")
	}
}

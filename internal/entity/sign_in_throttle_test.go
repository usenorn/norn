package entity_test

import (
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestSignInAttemptsAreSlowedOnlyAfterTheThirdFailure(t *testing.T) {
	cases := []struct {
		name     string
		failures int
		delay    time.Duration
	}{
		{"no failures", 0, 0},
		{"below the threshold", entity.SignInSlowdownAfter - 1, 0},
		{"at the threshold", entity.SignInSlowdownAfter, entity.SignInSlowdownStep},
		{"one past the threshold", entity.SignInSlowdownAfter + 1, 2 * entity.SignInSlowdownStep},
		{"far past the threshold", entity.SignInMaxFailures, entity.SignInSlowdownMax},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			throttle := entity.SignInThrottle{Failures: c.failures}

			if got := throttle.Delay(); got != c.delay {
				t.Fatalf("Delay() with %d failures = %s, want %s", c.failures, got, c.delay)
			}
		})
	}
}

func TestTheSlowdownNeverExceedsItsCap(t *testing.T) {
	throttle := entity.SignInThrottle{Failures: entity.SignInMaxFailures * 10}

	if got := throttle.Delay(); got != entity.SignInSlowdownMax {
		t.Fatalf("Delay() = %s, want the cap %s", got, entity.SignInSlowdownMax)
	}
}

func TestTheTenthConsecutiveFailureLeavesNoAttemptsLeft(t *testing.T) {
	throttle := entity.SignInThrottle{Failures: entity.SignInMaxFailures}

	if got := throttle.AttemptsLeft(); got != 0 {
		t.Fatalf("AttemptsLeft() = %d, want 0", got)
	}
}

func TestAttemptsLeftNeverGoesNegative(t *testing.T) {
	throttle := entity.SignInThrottle{Failures: entity.SignInMaxFailures + 5}

	if got := throttle.AttemptsLeft(); got != 0 {
		t.Fatalf("AttemptsLeft() = %d, want 0", got)
	}
}

func TestAttemptsLeftCountsDownFromTheMaximum(t *testing.T) {
	throttle := entity.SignInThrottle{Failures: 1}

	if got := throttle.AttemptsLeft(); got != entity.SignInMaxFailures-1 {
		t.Fatalf("AttemptsLeft() = %d, want %d", got, entity.SignInMaxFailures-1)
	}
}

func TestALockedThrottleStaysLockedUntilItsDeadline(t *testing.T) {
	unlocksAt := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		now    time.Time
		locked bool
	}{
		{"before", unlocksAt.Add(-time.Second), true},
		{"at the instant", unlocksAt, false},
		{"after", unlocksAt.Add(time.Second), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			throttle := entity.SignInThrottle{LockedUntil: unlocksAt}

			if got := throttle.Locked(c.now); got != c.locked {
				t.Fatalf("Locked(%s) = %v, want %v", c.now, got, c.locked)
			}
		})
	}
}

func TestAThrottleWithoutALockIsNeverLocked(t *testing.T) {
	if (entity.SignInThrottle{Failures: entity.SignInMaxFailures}).Locked(time.Now()) {
		t.Fatal("a throttle with no lock deadline reported itself locked")
	}
}

func TestTheSameAddressAlwaysHashesToTheSameSignInSubject(t *testing.T) {
	first := entity.HashSignInSubject("ada@example.com")
	second := entity.HashSignInSubject("ada@example.com")

	if first != second {
		t.Fatal("the same address produced two different subjects")
	}

	if first == entity.HashSignInSubject("grace@example.com") {
		t.Fatal("two different addresses produced the same subject")
	}

	if first == "ada@example.com" {
		t.Fatal("the subject leaks the raw address")
	}
}

func TestAnAccountLockedErrorUnwrapsToItsSentinel(t *testing.T) {
	err := entity.AccountLockedError{UnlocksAt: time.Now()}

	if unwrapped := err.Unwrap(); unwrapped != entity.ErrAccountLocked {
		t.Fatalf("Unwrap() = %v, want ErrAccountLocked", unwrapped)
	}
}

func TestAnInvalidCredentialsErrorUnwrapsToItsSentinel(t *testing.T) {
	err := entity.InvalidCredentialsError{AttemptsLeft: 3}

	if unwrapped := err.Unwrap(); unwrapped != entity.ErrAccountInvalidCredentials {
		t.Fatalf("Unwrap() = %v, want ErrAccountInvalidCredentials", unwrapped)
	}
}

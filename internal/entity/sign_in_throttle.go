package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	SignInMaxFailures   = 10
	SignInLockDuration  = 15 * time.Minute
	SignInFailureWindow = 15 * time.Minute

	SignInSlowdownAfter = 3
	SignInSlowdownStep  = 2 * time.Second
	SignInSlowdownMax   = 10 * time.Second

	SignInAddressMaxAttempts = 20
	SignInAddressWindow      = 2 * time.Minute
	SignInAddressCooldown    = 2 * time.Minute
)

var (
	ErrAccountLocked     = errors.New("account is temporarily locked")
	ErrSignInRateLimited = errors.New("too many sign-in attempts from this address")
)

type SignInThrottle struct {
	Failures    int
	LockedUntil time.Time
}

func (t SignInThrottle) Locked(now time.Time) bool {
	return !t.LockedUntil.IsZero() && now.Before(t.LockedUntil)
}

func (t SignInThrottle) AttemptsLeft() int {
	left := SignInMaxFailures - t.Failures
	if left < 0 {
		return 0
	}

	return left
}

func (t SignInThrottle) Delay() time.Duration {
	if t.Failures < SignInSlowdownAfter {
		return 0
	}

	delay := time.Duration(t.Failures-SignInSlowdownAfter+1) * SignInSlowdownStep
	if delay > SignInSlowdownMax {
		return SignInSlowdownMax
	}

	return delay
}

func HashSignInSubject(email string) string {
	sum := sha256.Sum256([]byte(email))

	return hex.EncodeToString(sum[:])
}

type AccountLockedError struct {
	UnlocksAt time.Time
}

func (e AccountLockedError) Error() string {
	return fmt.Sprintf("account is temporarily locked until %s", e.UnlocksAt.Format(time.RFC3339))
}

func (e AccountLockedError) Unwrap() error {
	return ErrAccountLocked
}

type InvalidCredentialsError struct {
	AttemptsLeft int
}

func (e InvalidCredentialsError) Error() string {
	return fmt.Sprintf("account credentials are invalid, %d attempts left", e.AttemptsLeft)
}

func (e InvalidCredentialsError) Unwrap() error {
	return ErrAccountInvalidCredentials
}

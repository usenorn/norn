package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	SignInChallengeTTL         = 10 * time.Minute
	SignInChallengeMaxAttempts = 5
	SignInCodeDigits           = 6
)

var (
	ErrSignInChallengeNotFound = errors.New("that sign-in has expired or was already finished")
	ErrSignInCodeIncorrect     = errors.New("that code is not the one we sent")
	ErrSignInCodeExhausted     = errors.New("that code was guessed at too many times")
)

type SignInChallenge struct {
	AccountID   uuid.UUID
	Email       string
	DisplayName string
	CodeHash    []byte
	Attempts    int
	IssuedAt    time.Time
	ExpiresAt   time.Time
	Client      SessionClient
}

func (c SignInChallenge) ExpiredAt(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}

func (c SignInChallenge) Exhausted() bool {
	return c.Attempts >= SignInChallengeMaxAttempts
}

func (c SignInChallenge) AttemptsLeft() int {
	left := SignInChallengeMaxAttempts - c.Attempts

	if left < 0 {
		return 0
	}

	return left
}

func (c SignInChallenge) Answers(code string) bool {
	return subtle.ConstantTimeCompare(c.CodeHash, HashSignInCode(code)) == 1
}

func NewSignInCode() (string, []byte, error) {
	limit := big.NewInt(1)

	for range SignInCodeDigits {
		limit.Mul(limit, big.NewInt(10))
	}

	drawn, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", nil, fmt.Errorf("generate sign-in code: %w", err)
	}

	digits := drawn.String()
	code := strings.Repeat("0", SignInCodeDigits-len(digits)) + digits

	return code, HashSignInCode(code), nil
}

func HashSignInCode(code string) []byte {
	sum := sha256.Sum256([]byte(NormalizeSignInCode(code)))

	return sum[:]
}

func NormalizeSignInCode(code string) string {
	return strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(code))
}

package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	SignUpTokenTTL   = time.Hour
	SignUpTokenBytes = 32
)

var (
	ErrSignUpNotFound         = errors.New("sign-up not found")
	ErrSignUpExpired          = errors.New("sign-up link expired")
	ErrSignUpAlreadyConfirmed = errors.New("sign-up link already used")
	ErrSignUpTokenInvalid     = errors.New("sign-up token is invalid")
	ErrSignUpsClosed          = errors.New("this instance does not accept sign-ups")
)

type SignUpDelivery string

const (
	SignUpDeliveryMailed   SignUpDelivery = "mailed"
	SignUpDeliveryLinkOnly SignUpDelivery = "link_only"
)

func (d SignUpDelivery) Valid() bool {
	switch d {
	case SignUpDeliveryMailed, SignUpDeliveryLinkOnly:
		return true
	default:
		return false
	}
}

type SignUp struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	Timezone     string
	PasswordHash string
	TokenHash    []byte
	RequestedAt  time.Time
	ExpiresAt    time.Time
	ConfirmedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (s SignUp) Confirmed() bool {
	return s.ConfirmedAt != nil
}

func (s SignUp) ExpiredAt(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

func NewSignUpToken() (string, []byte, error) {
	raw := make([]byte, SignUpTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate sign-up token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)

	return token, HashSignUpToken(token), nil
}

func HashSignUpToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

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
	EmailChangeTokenTTL   = 24 * time.Hour
	EmailChangeTokenBytes = 32
)

var (
	ErrEmailChangeNotFound     = errors.New("email change not found")
	ErrEmailChangeExpired      = errors.New("email change token expired")
	ErrEmailChangePending      = errors.New("email change already pending")
	ErrEmailChangeSameAddress  = errors.New("email change target matches the current address")
	ErrEmailChangeAlreadyDone  = errors.New("email change already confirmed")
	ErrEmailChangeTokenInvalid = errors.New("email change token is invalid")
)

type EmailChange struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	NewEmail    string
	TokenHash   []byte
	RequestedAt time.Time
	ExpiresAt   time.Time
	ConfirmedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (c EmailChange) Confirmed() bool {
	return c.ConfirmedAt != nil
}

func (c EmailChange) ExpiredAt(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}

func NewEmailChangeToken() (string, []byte, error) {
	raw := make([]byte, EmailChangeTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate email change token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)

	return token, HashEmailChangeToken(token), nil
}

func HashEmailChangeToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

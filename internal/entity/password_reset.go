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
	PasswordResetTokenTTL   = time.Hour
	PasswordResetTokenBytes = 32
)

var (
	ErrPasswordResetNotFound     = errors.New("password reset not found")
	ErrPasswordResetExpired      = errors.New("password reset link expired")
	ErrPasswordResetAlreadyUsed  = errors.New("password reset link already used")
	ErrPasswordResetTokenInvalid = errors.New("password reset token is invalid")
)

type PasswordReset struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	TokenHash   []byte
	RequestedAt time.Time
	ExpiresAt   time.Time
	UsedAt      *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r PasswordReset) Used() bool {
	return r.UsedAt != nil
}

func (r PasswordReset) ExpiredAt(now time.Time) bool {
	return !now.Before(r.ExpiresAt)
}

func NewPasswordResetToken() (string, []byte, error) {
	raw := make([]byte, PasswordResetTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate password reset token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)

	return token, HashPasswordResetToken(token), nil
}

func HashPasswordResetToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

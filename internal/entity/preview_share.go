package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	PreviewShareTokenPrefix = "nsl_"
	PreviewShareTokenBytes  = 32

	PreviewSharePasscodeMinLen = 6
	PreviewSharePasscodeMaxLen = 128

	PreviewShareLinksMax      = 20
	PreviewShareMaxAttempts   = 10
	PreviewShareAttemptWindow = 15 * time.Minute
)

var (
	ErrPreviewShareNotFound       = errors.New("this share link is not one norn minted")
	ErrPreviewShareExpired        = errors.New("this share link has expired")
	ErrPreviewShareRevoked        = errors.New("this share link has been revoked")
	ErrPreviewSharePasscode       = errors.New("that is not the passcode on this share link")
	ErrPreviewSharePasscodeNeeded = errors.New("this share link needs its passcode")
	ErrPreviewShareCrowded        = errors.New(
		"this preview already has as many share links as it may",
	)
	ErrPreviewShareGuessed = errors.New(
		"too many passcodes have been tried against this share link",
	)
)

type PreviewShareLink struct {
	ID           uuid.UUID
	PreviewID    uuid.UUID
	ExecutionID  string
	WorkspaceID  uuid.UUID
	TokenHash    []byte
	PasscodeHash string
	CreatedBy    uuid.UUID
	ExpiresAt    time.Time
	RevokedAt    time.Time
	LastUsedAt   time.Time
	Uses         int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (l PreviewShareLink) Revoked() bool {
	return !l.RevokedAt.IsZero()
}

func (l PreviewShareLink) Expired(now time.Time) bool {
	return !l.ExpiresAt.After(now)
}

func (l PreviewShareLink) NeedsPasscode() bool {
	return l.PasscodeHash != ""
}

func (l PreviewShareLink) Usable(now time.Time) error {
	switch {
	case l.Revoked():
		return ErrPreviewShareRevoked
	case l.Expired(now):
		return ErrPreviewShareExpired
	default:
		return nil
	}
}

func NewPreviewShareToken() (string, []byte, error) {
	raw := make([]byte, PreviewShareTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate preview share token: %w", err)
	}

	token := PreviewShareTokenPrefix + base64.RawURLEncoding.EncodeToString(raw)

	return token, HashPreviewShareToken(token), nil
}

func HashPreviewShareToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

func ValidatePreviewSharePasscode(field, passcode string) FieldError {
	if passcode == "" {
		return FieldError{}
	}

	switch length := utf8.RuneCountInString(strings.TrimSpace(passcode)); {
	case length < PreviewSharePasscodeMinLen:
		return FieldError{Field: field, Code: ValidationCodeTooShort}
	case length > PreviewSharePasscodeMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidatePreviewShareLifetime(field string, lifetime, longest time.Duration) FieldError {
	if lifetime <= 0 || lifetime > longest {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	return FieldError{}
}

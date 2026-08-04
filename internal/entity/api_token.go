package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	APITokenPrefix     = "nrn_"
	APITokenBytes      = 32
	APITokenNameMaxLen = 80
	APITokenMaxTTL     = 365 * 24 * time.Hour
)

var (
	ErrAPITokenNotFound      = errors.New("api token not found")
	ErrAPITokenNameTaken     = errors.New("api token name already used in this workspace")
	ErrAPITokenScopeInvalid  = errors.New("api token scope is not recognised")
	ErrAPITokenScopeExceeds  = errors.New("api token scope exceeds what its creator may do")
	ErrAPITokenMintForbidden = errors.New("api tokens may not mint or manage api tokens")
)

type APIToken struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	TokenHash   []byte
	Scopes      APIScopeSet
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (t APIToken) Revoked() bool {
	return t.RevokedAt != nil
}

func (t APIToken) ExpiredAt(now time.Time) bool {
	return t.ExpiresAt != nil && !now.Before(*t.ExpiresAt)
}

func (t APIToken) Usable(now time.Time) bool {
	return !t.Revoked() && !t.ExpiredAt(now)
}

func (t APIToken) NeedsUsageStamp(now time.Time, interval time.Duration) bool {
	return t.LastUsedAt == nil || now.Sub(*t.LastUsedAt) >= interval
}

func NewAPIToken() (string, []byte, error) {
	raw := make([]byte, APITokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate api token: %w", err)
	}

	token := APITokenPrefix + base64.RawURLEncoding.EncodeToString(raw)

	return token, HashAPIToken(token), nil
}

func HashAPIToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

func LooksLikeAPIToken(token string) bool {
	return strings.HasPrefix(token, APITokenPrefix)
}

func ValidateAPITokenName(field, name string) FieldError {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > APITokenNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	PreviewGatewaySecretPrefix = "ngr_"
	PreviewGatewayAccessPrefix = "nga_"
	PreviewGatewaySecretBytes  = 32
	PreviewGatewayNameMaxLen   = 200
)

var (
	ErrPreviewGatewayNotFound  = errors.New("preview gateway not found")
	ErrPreviewGatewayRevoked   = errors.New("this preview gateway has been revoked")
	ErrPreviewGatewayNameTaken = errors.New(
		"a preview gateway already answers to that name",
	)
	ErrPreviewGatewayCredentialInvalid = errors.New(
		"preview gateway credential is not valid",
	)
)

type PreviewGatewayStatus string

const (
	PreviewGatewayActive  PreviewGatewayStatus = "active"
	PreviewGatewayRevoked PreviewGatewayStatus = "revoked"
)

func PreviewGatewayStatuses() []PreviewGatewayStatus {
	return []PreviewGatewayStatus{PreviewGatewayActive, PreviewGatewayRevoked}
}

func (s PreviewGatewayStatus) Valid() bool {
	return slices.Contains(PreviewGatewayStatuses(), s)
}

type PreviewGateway struct {
	ID         uuid.UUID
	Name       string
	Status     PreviewGatewayStatus
	LastSeenAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewPreviewGatewaySecret(prefix string) (string, []byte, error) {
	raw := make([]byte, PreviewGatewaySecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate preview gateway secret: %w", err)
	}

	secret := prefix + base64.RawURLEncoding.EncodeToString(raw)

	return secret, HashPreviewGatewaySecret(secret), nil
}

func HashPreviewGatewaySecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))

	return sum[:]
}

func LooksLikePreviewGatewayToken(token string) bool {
	return strings.HasPrefix(token, PreviewGatewayAccessPrefix)
}

func ValidatePreviewGatewayName(field, name string) FieldError {
	return requiredText(field, name, PreviewGatewayNameMaxLen)
}

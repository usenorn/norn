package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=api_token.go -destination=apitoken/mock_api_token.go -package=apitoken -mock_names=APIToken=MockAPIToken

type APIToken interface {
	Create(ctx context.Context, token entity.APIToken) (entity.APIToken, error)
	GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.APIToken, error)
	GetByID(ctx context.Context, tokenID uuid.UUID) (entity.APIToken, error)
	ListByOwner(ctx context.Context, accountID uuid.UUID) ([]entity.APIToken, error)
	ListByWorkspaceGrant(ctx context.Context, workspaceID uuid.UUID) ([]entity.APIToken, error)
	Revoke(ctx context.Context, tokenID uuid.UUID, revokedAt time.Time) error
	RecordUsage(ctx context.Context, tokenID uuid.UUID, usedAt time.Time) error
	ListExpiring(ctx context.Context, notAfter time.Time) ([]entity.APIToken, error)
	RecordExpiryNotice(ctx context.Context, tokenID uuid.UUID, days int) error
}

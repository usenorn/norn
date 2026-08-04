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
	ListByOwner(ctx context.Context, workspaceID, accountID uuid.UUID) ([]entity.APIToken, error)
	Revoke(ctx context.Context, workspaceID, accountID, tokenID uuid.UUID, revokedAt time.Time) error
	RecordUsage(ctx context.Context, tokenID uuid.UUID, usedAt time.Time) error
}

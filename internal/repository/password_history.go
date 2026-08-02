package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=password_history.go -destination=passwordhistory/mock_password_history.go -package=passwordhistory -mock_names=PasswordHistory=MockPasswordHistory

type PasswordHistory interface {
	Create(ctx context.Context, entry entity.PasswordHistoryEntry) (entity.PasswordHistoryEntry, error)
	ListRecentByAccountID(ctx context.Context, accountID uuid.UUID, limit int) ([]entity.PasswordHistoryEntry, error)
	PruneByAccountID(ctx context.Context, accountID uuid.UUID, keep int) error
	DeleteByAccountID(ctx context.Context, accountID uuid.UUID) error
}

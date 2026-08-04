package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=account.go -destination=account/mock_account.go -package=account -mock_names=Account=MockAccount

type Account interface {
	Create(ctx context.Context, account entity.Account) (entity.Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (entity.Account, error)
	GetByEmail(ctx context.Context, email string) (entity.Account, error)
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]entity.Account, error)
	Update(ctx context.Context, account entity.Account) (entity.Account, error)
}

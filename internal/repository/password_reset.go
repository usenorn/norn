package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=password_reset.go -destination=passwordreset/mock_password_reset.go -package=passwordreset -mock_names=PasswordReset=MockPasswordReset

type PasswordReset interface {
	Create(ctx context.Context, reset entity.PasswordReset) (entity.PasswordReset, error)
	GetByID(ctx context.Context, id uuid.UUID) (entity.PasswordReset, error)
	GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.PasswordReset, error)
	MarkUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error
	DeletePendingByAccountID(ctx context.Context, accountID uuid.UUID) error
}

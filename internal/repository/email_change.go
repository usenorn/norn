package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=email_change.go -destination=emailchange/mock_email_change.go -package=emailchange -mock_names=EmailChange=MockEmailChange

type EmailChange interface {
	Create(ctx context.Context, change entity.EmailChange) (entity.EmailChange, error)
	GetByID(ctx context.Context, id uuid.UUID) (entity.EmailChange, error)
	GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.EmailChange, error)
	GetPendingByAccountID(ctx context.Context, accountID uuid.UUID) (entity.EmailChange, error)
	MarkConfirmed(ctx context.Context, id uuid.UUID, confirmedAt time.Time) error
	DeletePendingByAccountID(ctx context.Context, accountID uuid.UUID) error
}

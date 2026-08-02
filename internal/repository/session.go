package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=session.go -destination=session/mock_session.go -package=session -mock_names=Session=MockSession

type Session interface {
	Create(ctx context.Context, session entity.Session) error
	Get(ctx context.Context, tokenHash string) (entity.Session, error)
	Touch(ctx context.Context, session entity.Session) error
	ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]entity.Session, error)
	Delete(ctx context.Context, tokenHash string) error
	DeleteByID(ctx context.Context, accountID, sessionID uuid.UUID) error
	DeleteByAccountID(ctx context.Context, accountID uuid.UUID) error
	DeleteByAccountIDExcept(ctx context.Context, accountID uuid.UUID, keepTokenHash string) error
	RevokedAt(ctx context.Context, accountID uuid.UUID) (time.Time, error)
	MarkRevoked(ctx context.Context, accountID uuid.UUID, at time.Time) error
}

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=sign_up.go -destination=signup/mock_sign_up.go -package=signup -mock_names=SignUp=MockSignUp

type SignUp interface {
	Create(ctx context.Context, signUp entity.SignUp) (entity.SignUp, error)
	GetByID(ctx context.Context, id uuid.UUID) (entity.SignUp, error)
	GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.SignUp, error)
	MarkConfirmed(ctx context.Context, id uuid.UUID, confirmedAt time.Time) error
	DeletePendingByEmail(ctx context.Context, email string) error
}

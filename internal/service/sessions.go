package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=sessions.go -destination=session/mock_sessions.go -package=session -mock_names=Sessions=MockSessions

type Sessions interface {
	SignIn(ctx context.Context, input SignInInput) (IssuedSession, error)
	Validate(ctx context.Context, token string) (entity.Session, error)
	SignOut(ctx context.Context, sessionID uuid.UUID) error
	List(ctx context.Context, accountID uuid.UUID) ([]entity.Session, error)
	Revoke(ctx context.Context, accountID, sessionID uuid.UUID) error
	RevokeAllByAccountID(ctx context.Context, accountID uuid.UUID) error
	RotateAfterCredentialChange(ctx context.Context, accountID uuid.UUID) (IssuedSession, error)
}

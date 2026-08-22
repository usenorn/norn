package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=sessions.go -destination=session/mock_sessions.go -package=session -mock_names=Sessions=MockSessions

type Sessions interface {
	SignIn(ctx context.Context, input SignInInput) (IssuedChallenge, error)
	VerifySignInCode(ctx context.Context, input VerifySignInCodeInput) (IssuedSession, error)
	SendSignInCode(ctx context.Context, challengeID, code string) error
	Start(ctx context.Context, input StartSessionInput) (IssuedSession, error)
	Validate(ctx context.Context, token string) (entity.Session, error)
	Inspect(ctx context.Context, token string) (entity.Session, error)
	Resolve(ctx context.Context, input ResolveSessionsInput) (ResolvedSessions, error)
	SignOut(ctx context.Context, sessionID uuid.UUID) error
	SignOutAll(ctx context.Context) error
	List(ctx context.Context, accountID uuid.UUID) ([]entity.Session, error)
	Revoke(ctx context.Context, accountID, sessionID uuid.UUID) error
	RevokeAllByAccountID(ctx context.Context, accountID uuid.UUID) error
	RotateAfterCredentialChange(ctx context.Context, accountID uuid.UUID) (IssuedSession, error)
}

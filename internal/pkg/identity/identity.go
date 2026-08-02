package identity

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type (
	accountKey struct{}
	sessionKey struct{}
)

func Into(ctx context.Context, accountID uuid.UUID) context.Context {
	return context.WithValue(ctx, accountKey{}, accountID)
}

func From(ctx context.Context) (uuid.UUID, bool) {
	accountID, ok := ctx.Value(accountKey{}).(uuid.UUID)

	return accountID, ok
}

func WithSession(ctx context.Context, session entity.Session) context.Context {
	return Into(context.WithValue(ctx, sessionKey{}, session), session.AccountID)
}

func CurrentSession(ctx context.Context) (entity.Session, bool) {
	session, ok := ctx.Value(sessionKey{}).(entity.Session)

	return session, ok
}

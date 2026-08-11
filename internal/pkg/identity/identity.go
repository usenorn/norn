package identity

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type (
	actorKey    struct{}
	sessionKey  struct{}
	signedInKey struct{}
	approvalKey struct{}
	clientKey   struct{}
)

func WithClient(ctx context.Context, client entity.SessionClient) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

func Client(ctx context.Context) entity.SessionClient {
	client, _ := ctx.Value(clientKey{}).(entity.SessionClient)

	return client
}

func WithApproval(ctx context.Context, approver uuid.UUID) context.Context {
	return context.WithValue(ctx, approvalKey{}, approver)
}

func Approver(ctx context.Context) (uuid.UUID, bool) {
	approver, _ := ctx.Value(approvalKey{}).(uuid.UUID)

	return approver, approver != uuid.Nil
}

func Approved(ctx context.Context) bool {
	_, approved := Approver(ctx)

	return approved
}

func WithActor(ctx context.Context, actor entity.Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func Actor(ctx context.Context) (entity.Actor, bool) {
	actor, ok := ctx.Value(actorKey{}).(entity.Actor)
	if !ok || actor.Anonymous() {
		return entity.Actor{}, false
	}

	return actor, true
}

func Into(ctx context.Context, accountID uuid.UUID) context.Context {
	return WithActor(ctx, entity.Actor{Kind: entity.ActorKindUser, AccountID: accountID})
}

func From(ctx context.Context) (uuid.UUID, bool) {
	actor, ok := Actor(ctx)
	if !ok {
		return uuid.Nil, false
	}

	return actor.AccountID, true
}

func WithSession(ctx context.Context, session entity.Session) context.Context {
	return WithActor(context.WithValue(ctx, sessionKey{}, session), entity.UserActor(session))
}

func CurrentSession(ctx context.Context) (entity.Session, bool) {
	session, ok := ctx.Value(sessionKey{}).(entity.Session)

	return session, ok
}

func WithSignedIn(ctx context.Context, sessions []entity.Session) context.Context {
	return context.WithValue(ctx, signedInKey{}, sessions)
}

func SignedIn(ctx context.Context) []entity.Session {
	sessions, _ := ctx.Value(signedInKey{}).([]entity.Session)

	return sessions
}

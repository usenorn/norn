package service

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type SignInInput struct {
	Email    string
	Password string
	Client   entity.SessionClient
}

type StartSessionInput struct {
	AccountID   uuid.UUID
	WorkspaceID uuid.UUID
	AuthMethod  entity.SessionAuthMethod
	Client      entity.SessionClient
}

type IssuedSession struct {
	Session entity.Session
	Token   string
}

type PresentedSession struct {
	Slot  string
	Token string
}

type ResolveSessionsInput struct {
	Presented   []PresentedSession
	Selector    string
	WorkspaceID uuid.UUID
}

type ResolvedSessions struct {
	Held   []entity.Session
	Acting entity.Session
	Found  bool
	Dead   []string
}

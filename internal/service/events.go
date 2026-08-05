package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=events.go -destination=event/mock_events.go -package=event -mock_names=Events=MockEvents

type SubscribeInput struct {
	WorkspaceID  uuid.UUID
	Subscription entity.EventSubscription
	Cursor       string
}

type Subscription struct {
	Events <-chan entity.Event
	Missed []entity.Event
	Lapsed bool
	Close  func()
}

type Events interface {
	Publish(ctx context.Context, event entity.Event)
	Subscribe(ctx context.Context, input SubscribeInput) (*Subscription, error)
}

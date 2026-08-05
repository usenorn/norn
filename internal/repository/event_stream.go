package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=event_stream.go -destination=eventstream/mock_event_stream.go -package=eventstream -mock_names=EventStream=MockEventStream

type EventStream interface {
	Publish(ctx context.Context, event entity.Event) error
	Since(ctx context.Context, workspaceID uuid.UUID, cursor string) ([]entity.Event, error)
	Read(ctx context.Context, workspaceID uuid.UUID, cursor string) ([]entity.Event, string, error)
	Latest(ctx context.Context, workspaceID uuid.UUID) (string, error)
}

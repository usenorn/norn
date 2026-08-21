package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=runner_channel.go -destination=runnerchannel/mock_runner_channel.go -package=runnerchannel -mock_names=RunnerChannel=MockRunnerChannel

type RunnerChannel interface {
	Append(ctx context.Context, runnerID uuid.UUID, message entity.ChannelMessage) (string, error)
	Read(
		ctx context.Context, runnerID uuid.UUID, cursor string,
	) ([]entity.SpooledMessage, string, error)
	Cursor(ctx context.Context, runnerID uuid.UUID) (string, error)
	Acknowledge(ctx context.Context, runnerID uuid.UUID, cursor string) error

	Attach(ctx context.Context, runnerID uuid.UUID, epoch string, seenAt time.Time) error
	Renew(ctx context.Context, runnerID uuid.UUID, epoch string, seenAt time.Time) error
	Detach(ctx context.Context, runnerID uuid.UUID, epoch string) error
	Presence(ctx context.Context, runnerID uuid.UUID) (entity.RunnerPresence, error)

	Seen(ctx context.Context, runnerID uuid.UUID, messageID string) (bool, error)
}

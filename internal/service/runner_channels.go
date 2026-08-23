package service

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=runner_channels.go -destination=runnerchannel/mock_runner_channels.go -package=runnerchannel -mock_names=RunnerChannels=MockRunnerChannels

type ChannelSession struct {
	Runner entity.Runner
	Actor  entity.Actor
	Epoch  string
	Cursor string
}

type RunnerChannels interface {
	Open(ctx context.Context, ticket string) (ChannelSession, error)
	Deliver(
		ctx context.Context, session ChannelSession, cursor string,
	) ([]entity.SpooledMessage, string, error)
	Receive(ctx context.Context, session ChannelSession, message entity.ChannelMessage) error
	Acknowledge(ctx context.Context, session ChannelSession, cursor string) error
	Heartbeat(ctx context.Context, session ChannelSession, load entity.RunnerLoad) error
	Verify(ctx context.Context, session ChannelSession) error
	Close(ctx context.Context, session ChannelSession)
}

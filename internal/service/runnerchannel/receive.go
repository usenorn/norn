package runnerchannel

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *channelsService) greet(
	ctx context.Context,
	session service.ChannelSession,
	message entity.ChannelMessage,
) {
	var hello channelv1.Hello

	if err := json.Unmarshal(message.Payload, &hello); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"a runner said hello in a shape this server could not read",
			slog.String("runner_id", session.Runner.ID.String()),
			slog.String("error", err.Error()),
		)

		return
	}

	if hello.Version == "" || hello.Version == session.Runner.Host.Version {
		return
	}

	if err := s.runners.RecordVersion(ctx, session.Runner.ID, hello.Version); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"a runner named a new version this server could not record",
			slog.String("runner_id", session.Runner.ID.String()),
			slog.String("version", hello.Version),
			slog.String("error", err.Error()),
		)
	}
}

func (s *channelsService) Receive(
	ctx context.Context,
	session service.ChannelSession,
	message entity.ChannelMessage,
) error {
	if err := entity.ValidateChannelInbound(message); err != nil {
		return err
	}

	spent, err := s.channels.Seen(ctx, session.Runner.ID, message.ID)
	if err != nil {
		return err
	}

	if spent {
		return nil
	}

	ctx = identity.WithActor(ctx, session.Actor)

	switch message.Type {
	case entity.ChannelRunnerHello:
		s.greet(ctx, session, message)

		return s.Heartbeat(ctx, session)
	case entity.ChannelRunnerHeartbeat:
		return s.Heartbeat(ctx, session)
	case entity.ChannelExecutionAccepted:
		return s.executions.Accepted(ctx, session.Runner, message)
	case entity.ChannelExecutionDeclined:
		return s.executions.Declined(ctx, session.Runner, message)
	case entity.ChannelExecutionState:
		return s.executions.Reported(ctx, session.Runner, message)
	case entity.ChannelExecutionEvent:
		return s.executions.Observed(ctx, session.Runner, message)
	default:
		logging.From(ctx).InfoContext(
			ctx,
			"a runner sent a message this server does not act on yet",
			slog.String("runner_id", session.Runner.ID.String()),
			slog.String("type", string(message.Type)),
			slog.String("message_id", message.ID),
		)

		return nil
	}
}

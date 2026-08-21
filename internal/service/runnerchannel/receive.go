package runnerchannel

import (
	"context"
	"log/slog"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

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

	switch message.Type {
	case entity.ChannelRunnerHello, entity.ChannelRunnerHeartbeat:
		return s.Heartbeat(ctx, session)
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

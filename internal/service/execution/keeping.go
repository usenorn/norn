package execution

import (
	"context"
	"encoding/json"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *executionsService) Kept(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var reported channelv1.Retention

	if len(message.Payload) > 0 {
		if err := json.Unmarshal(message.Payload, &reported); err != nil {
			return entity.ErrChannelEnvelopeInvalid
		}
	}

	if reported.KeepUntil.IsZero() {
		return nil
	}

	keepUntil := reported.KeepUntil.UTC()

	if execution.KeepUntil != nil && execution.KeepUntil.Equal(keepUntil) {
		return nil
	}

	kept, err := s.executions.Keep(ctx, execution.ID, keepUntil)
	if err != nil {
		return err
	}

	s.broadcast(ctx, kept)

	return nil
}

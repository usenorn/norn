package execution

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *executionsService) held(
	ctx context.Context,
	runner entity.Runner,
	executionID string,
) (entity.Execution, error) {
	if executionID == "" {
		return entity.Execution{}, entity.ErrExecutionNotFound
	}

	execution, err := s.executions.GetByID(ctx, executionID)
	if err != nil {
		return entity.Execution{}, err
	}

	if execution.RunnerID != runner.ID {
		return entity.Execution{}, entity.ErrExecutionNotFound
	}

	return execution, nil
}

func (s *executionsService) Held(
	ctx context.Context,
	runner entity.Runner,
	executionID string,
) (entity.Execution, error) {
	return s.held(ctx, runner, executionID)
}

func (s *executionsService) Accepted(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	leased, err := s.advance(ctx, execution, move{
		to:         entity.ExecutionLeased,
		actor:      runnerActor(runner),
		runnerID:   runner.ID,
		sourceID:   message.ID,
		occurredAt: message.IssuedAt,
	})
	if err != nil {
		return err
	}

	return s.tell(ctx, leased, entity.ChannelExecutionStart, channelv1.Start{
		ExecutionID:    leased.ID,
		LeaseExpiresAt: leased.LeaseExpiresAt,
		Params:         paramsOf(leased.Params),
	})
}

func (s *executionsService) Declined(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var declined channelv1.Decline
	if err := decode(message.Payload, &declined); err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		return s.remember(ctx, execution, entity.ExecutionEvent{
			ExecutionID: execution.ID,
			Kind:        entity.ExecutionEventNote,
			Actor:       runnerActor(runner),
			Reason:      declined.Reason,
			SourceID:    message.ID,
			OccurredAt:  occurredAt(message),
		})
	})
}

func (s *executionsService) Reported(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var reported channelv1.Report
	if err := decode(message.Payload, &reported); err != nil {
		return err
	}

	target := entity.ExecutionState(reported.State)

	if !target.Valid() {
		return entity.ErrExecutionTransition
	}

	if !target.RunnerDriven() {
		return entity.ErrExecutionStateNotRunners
	}

	occurred := reported.Occurred
	if occurred.IsZero() {
		occurred = occurredAt(message)
	}

	_, err = s.advance(ctx, execution, move{
		to:         target,
		reason:     firstOf(reported.Reason, reported.Detail),
		actor:      runnerActor(runner),
		sourceID:   message.ID,
		occurredAt: occurred,
	})

	return err
}

func (s *executionsService) Observed(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var observed channelv1.Entry
	if err := decode(message.Payload, &observed); err != nil {
		return err
	}

	kind := entity.ExecutionEventKind(observed.Kind)
	if !kind.Valid() {
		kind = entity.ExecutionEventNote
	}

	occurred := observed.Occurred
	if occurred.IsZero() {
		occurred = occurredAt(message)
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		return s.remember(ctx, execution, entity.ExecutionEvent{
			ExecutionID: execution.ID,
			Kind:        kind,
			Actor:       runnerActor(runner),
			Reason:      observed.Reason,
			Detail:      objectOrNothing(observed.Detail),
			SourceID:    message.ID,
			OccurredAt:  occurred,
		})
	})
}

func (s *executionsService) Renew(ctx context.Context, runner entity.Runner) error {
	return s.executions.RenewLeases(ctx, runner.ID, time.Now().UTC().Add(entity.ChannelLeaseTTL))
}

func (s *executionsService) Leased(
	ctx context.Context,
	runnerID uuid.UUID,
) ([]string, error) {
	live, err := s.executions.ListLiveByRunner(ctx, runnerID)
	if err != nil {
		return nil, err
	}

	held := make([]string, 0, len(live))
	for _, execution := range live {
		held = append(held, execution.ID)
	}

	return held, nil
}

func (s *executionsService) SweepLeases(ctx context.Context) error {
	now := time.Now().UTC()

	lapsed, err := s.executions.ExpiredLeases(ctx, now, sweepBatch)
	if err != nil {
		return err
	}

	for _, execution := range lapsed {
		if _, err := s.advance(ctx, execution, move{
			to:     entity.ExecutionInterrupted,
			reason: entity.ErrExecutionLeaseLapsed.Error(),
			actor:  entity.SystemExecutionActor(),
		}); err != nil {
			if errors.Is(err, entity.ErrExecutionTransition) {
				continue
			}

			return err
		}

		if err := s.previews.CloseByExecution(ctx, execution.ID, now); err != nil {
			return err
		}

		logging.From(ctx).InfoContext(
			ctx,
			"an execution was interrupted because its runner stopped reporting",
			slog.String("execution_id", execution.ID),
			slog.String("runner_id", execution.RunnerID.String()),
		)
	}

	return nil
}

func runnerActor(runner entity.Runner) entity.ExecutionActor {
	return entity.ExecutionActor{
		Kind:     entity.ActorKindAgent,
		AgentID:  runner.AgentID,
		RunnerID: runner.ID,
	}
}

func occurredAt(message entity.ChannelMessage) time.Time {
	if message.IssuedAt.IsZero() {
		return time.Now().UTC()
	}

	return message.IssuedAt
}

func decode(payload []byte, into any) error {
	if len(payload) == 0 {
		return nil
	}

	if err := json.Unmarshal(payload, into); err != nil {
		return entity.ErrChannelEnvelopeInvalid
	}

	return nil
}

func objectOrNothing(detail json.RawMessage) []byte {
	var decoded map[string]any

	if json.Unmarshal(detail, &decoded) != nil || decoded == nil {
		return nil
	}

	return detail
}

func firstOf(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

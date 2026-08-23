package execution

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *executionsService) Retain(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	longer time.Duration,
) (entity.Execution, error) {
	decision, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionManage)
	if err != nil {
		return entity.Execution{}, err
	}

	if execution.State != entity.ExecutionAwaitingReview {
		return entity.Execution{}, entity.ErrPreviewNotExtendable
	}

	if longer <= 0 || longer > entity.PreviewRetentionLongest {
		return entity.Execution{}, entity.NewValidationError(entity.FieldError{
			Field: "longer",
			Code:  entity.ValidationCodeOutOfRange,
		})
	}

	if execution.RunnerID == uuid.Nil {
		return entity.Execution{}, entity.ErrExecutionNoRunner
	}

	keepUntil := time.Now().UTC().Add(longer)

	if err := s.tell(ctx, execution, entity.ChannelExecutionRetain, channelv1.Retention{
		KeepUntil: keepUntil,
	}); err != nil {
		return entity.Execution{}, err
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		return s.remember(ctx, execution, entity.ExecutionEvent{
			ExecutionID: execution.ID,
			Kind:        entity.ExecutionEventPhase,
			Actor:       entity.ExecutionActorOf(decision.Actor),
			Reason: "this run keeps its workspace and its previews until " +
				keepUntil.Format(time.RFC3339),
			OccurredAt: time.Now().UTC(),
		})
	}); err != nil {
		return entity.Execution{}, err
	}

	s.record(ctx, entity.AuditExecutionRetained, execution)

	return execution, nil
}

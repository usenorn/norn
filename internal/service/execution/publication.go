package execution

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *executionsService) RetryPublication(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) (entity.Execution, error) {
	return s.handPublication(ctx, workspaceID, executionID, channelv1.ResumePublish,
		entity.AuditExecutionRepublish)
}

func (s *executionsService) AbandonPublication(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) (entity.Execution, error) {
	return s.handPublication(ctx, workspaceID, executionID, channelv1.ResumeAbandon,
		entity.AuditExecutionAbandoned)
}

func (s *executionsService) handPublication(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	reason string,
	action entity.AuditAction,
) (entity.Execution, error) {
	decision, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionManage)
	if err != nil {
		return entity.Execution{}, err
	}

	if execution.State != entity.ExecutionApproved || execution.RunnerID == uuid.Nil {
		return entity.Execution{}, entity.ErrPublicationNotPending
	}

	if err := s.authorised(ctx, decision, execution); err != nil {
		return entity.Execution{}, err
	}

	reviews, err := s.reviews.ListReviews(ctx, execution.ID)
	if err != nil {
		return entity.Execution{}, err
	}

	approval, ok := entity.LastApproval(reviews)
	if !ok {
		return entity.Execution{}, entity.ErrPublicationNotPending
	}

	instruction := publishing(reason, execution, approval.Revision, approval.Heads)

	if err := s.tell(ctx, execution, entity.ChannelExecutionResume, instruction); err != nil {
		return entity.Execution{}, err
	}

	s.record(ctx, action, execution)

	return execution, nil
}

package execution

import (
	"context"
	"log/slog"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

func (s *executionsService) authorised(
	ctx context.Context,
	decision entity.Decision,
	execution entity.Execution,
) error {
	authority, err := s.delegates.Authority(ctx, execution.WorkspaceID, execution.IssueID)
	if err != nil {
		return err
	}

	if !authority.Permits(decision) {
		return entity.ErrIssueDecisionForbidden
	}

	return nil
}

func (s *executionsService) relay(
	ctx context.Context,
	decision entity.TelegramDecision,
	enqueue func(context.Context, entity.TelegramDecision) error,
) {
	postgres.AfterCommit(ctx, func(ctx context.Context) {
		if err := enqueue(ctx, decision); err != nil {
			logging.From(ctx).WarnContext(
				ctx, "queueing a decision for telegram failed",
				slog.String("execution_id", decision.ExecutionID), slog.String("error", err.Error()),
			)
		}
	})
}

func (s *executionsService) relayWaiting(ctx context.Context, execution entity.Execution) error {
	if execution.State == entity.ExecutionAwaitingPlan {
		plans, err := s.plans.ListByExecution(ctx, execution.ID)
		if err != nil {
			return err
		}

		if latest, ok := entity.LatestPlan(plans); ok {
			s.relay(ctx, entity.TelegramPlanDecision(execution, latest.Revision), s.jobs.EnqueueTelegramDecision)
		}

		return nil
	}

	latest, err := s.latest(ctx, execution)
	if err != nil {
		return err
	}

	s.relay(ctx, entity.TelegramReviewDecision(execution, latest.Heads()), s.jobs.EnqueueTelegramDecision)

	return nil
}

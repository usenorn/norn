package telegrambot

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

const replyForChanges = "Reply to this message with what should change."

func (s *updates) pressed(
	ctx context.Context,
	account entity.TelegramAccount,
	decision entity.TelegramDecisionMessage,
	data string,
) (string, error) {
	if decision.Kind == entity.TelegramDecisionPublication {
		return s.publishing(ctx, account, decision, data)
	}

	switch data {
	case entity.TelegramCallbackChanges:
		return replyForChanges, nil
	case entity.TelegramCallbackApprove:
	default:
		return "That option is not available.", nil
	}

	execution, err := s.executions.GetByID(ctx, decision.ExecutionID)
	if err != nil {
		return "", err
	}

	if decision.Kind == entity.TelegramDecisionPlan {
		err = s.transactor.WithSavepoint(ctx, func(ctx context.Context) error {
			_, err := s.decisions.ApprovePlan(
				identity.WithActor(ctx, account.Actor()), execution.WorkspaceID, execution.ID, decision.PlanRevision,
			)

			return err
		})

		return s.outcome(ctx, execution.WorkspaceID, execution.IssueID, err, "Plan approved.")
	}

	err = s.transactor.WithSavepoint(ctx, func(ctx context.Context) error {
		_, err := s.decisions.SubmitReview(
			identity.WithActor(ctx, account.Actor()), execution.WorkspaceID, execution.ID,
			service.ReviewSubmission{Verdict: entity.VerdictApprove, Heads: decision.ReviewHeads},
		)

		return err
	})

	return s.outcome(ctx, execution.WorkspaceID, execution.IssueID, err, "Changes approved.")
}

func (s *updates) publishing(
	ctx context.Context,
	account entity.TelegramAccount,
	decision entity.TelegramDecisionMessage,
	data string,
) (string, error) {
	hand, done := s.decisions.RetryPublication, "Retrying publication."

	switch data {
	case entity.TelegramCallbackRetry:
	case entity.TelegramCallbackAbandon:
		hand, done = s.decisions.AbandonPublication, "Publication abandoned."
	default:
		return "That option is not available.", nil
	}

	execution, err := s.executions.GetByID(ctx, decision.ExecutionID)
	if err != nil {
		return "", err
	}

	err = s.transactor.WithSavepoint(ctx, func(ctx context.Context) error {
		_, err := hand(identity.WithActor(ctx, account.Actor()), execution.WorkspaceID, execution.ID)

		return err
	})

	return s.outcome(ctx, execution.WorkspaceID, execution.IssueID, err, done)
}

func (s *updates) feedback(
	ctx context.Context,
	account entity.TelegramAccount,
	decision entity.TelegramDecisionMessage,
	text string,
) (string, error) {
	if decision.Settled {
		return "This was already decided.", nil
	}

	if decision.Kind == entity.TelegramDecisionPublication {
		return "Tap Retry publication or Give up.", nil
	}

	execution, err := s.executions.GetByID(ctx, decision.ExecutionID)
	if err != nil {
		return "", err
	}

	if decision.Kind == entity.TelegramDecisionPlan {
		err = s.transactor.WithSavepoint(ctx, func(ctx context.Context) error {
			_, err := s.decisions.RevisePlan(
				identity.WithActor(ctx, account.Actor()),
				execution.WorkspaceID, execution.ID, decision.PlanRevision, text,
			)

			return err
		})

		return s.outcome(ctx, execution.WorkspaceID, execution.IssueID, err, "Sent back for a new plan.")
	}

	err = s.transactor.WithSavepoint(ctx, func(ctx context.Context) error {
		_, err := s.decisions.SubmitReview(
			identity.WithActor(ctx, account.Actor()), execution.WorkspaceID, execution.ID,
			service.ReviewSubmission{
				Verdict: entity.VerdictRequestChanges,
				Summary: text,
				Heads:   decision.ReviewHeads,
			},
		)

		return err
	})

	return s.outcome(ctx, execution.WorkspaceID, execution.IssueID, err, "Changes requested.")
}

func (s *updates) outcome(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	err error,
	done string,
) (string, error) {
	var (
		invalid entity.ValidationError
		denied  entity.AccessDeniedError
	)

	switch {
	case err == nil:
		return done, nil
	case errors.Is(err, entity.ErrIssueDecisionForbidden):
		return s.forbidden(ctx, workspaceID, issueID), nil
	case errors.Is(err, entity.ErrExecutionPlanStale), errors.Is(err, entity.ErrReviewStale):
		return "This is out of date. Open it in Norn to see the latest.", nil
	case errors.Is(err, entity.ErrIssueQuestionAnswered), errors.Is(err, entity.ErrIssueQuestionSettled),
		errors.Is(err, entity.ErrExecutionNotPlanning), errors.Is(err, entity.ErrExecutionPlanMissing),
		errors.Is(err, entity.ErrReviewClosed), errors.Is(err, entity.ErrExecutionTransition),
		errors.Is(err, entity.ErrPublicationNotPending):
		return "This was already decided.", nil
	case errors.Is(err, entity.ErrExecutionQuestionsOpen):
		return "The run is still waiting on an answer. Answer its open questions first.", nil
	case errors.Is(err, entity.ErrReviewEmpty), errors.As(err, &invalid):
		return "Say what should change.", nil
	case errors.Is(err, entity.ErrExecutionSelfApproval):
		return "An agent may not decide on its own work.", nil
	case errors.Is(err, entity.ErrIssueQuestionNotFound), errors.Is(err, entity.ErrIssueNotFound),
		errors.Is(err, entity.ErrExecutionNotFound), errors.Is(err, entity.ErrAccountForbidden),
		errors.As(err, &denied):
		return "You cannot decide on this issue.", nil
	default:
		return "", err
	}
}

func (s *updates) forbidden(ctx context.Context, workspaceID, issueID uuid.UUID) string {
	authority, err := s.delegations.Authority(ctx, workspaceID, issueID)
	if err != nil || authority.MakerName() == "" {
		return "Only the assignee or a workspace admin can decide this."
	}

	return fmt.Sprintf("Only %s or a workspace admin can decide this.", authority.MakerName())
}

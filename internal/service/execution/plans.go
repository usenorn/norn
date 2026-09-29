package execution

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *executionsService) Plans(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) ([]entity.ExecutionPlan, error) {
	_, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	return s.plans.ListByExecution(ctx, execution.ID)
}

func (s *executionsService) PlanProposed(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var proposed channelv1.Plan
	if err := decode(message.Payload, &proposed); err != nil {
		return err
	}

	if err := entity.NewValidationError(
		entity.ValidateExecutionPlanRef("ref", proposed.Ref),
		entity.ValidateExecutionPlanBody("body", proposed.Body),
	); err != nil {
		return err
	}

	if execution.Stage != entity.StagePlanning {
		return entity.ErrExecutionNotPlanning
	}

	occurred := proposed.Proposed
	if occurred.IsZero() {
		occurred = occurredAt(message)
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		plan, err := s.plans.Propose(ctx, entity.ExecutionPlan{
			ExecutionID: execution.ID,
			WorkspaceID: execution.WorkspaceID,
			Ref:         strings.TrimSpace(proposed.Ref),
			Body:        strings.TrimSpace(proposed.Body),
			ProposedAt:  occurred,
		})
		if errors.Is(err, entity.ErrExecutionPlanRecorded) {
			return nil
		}

		if err != nil {
			return err
		}

		if err := s.remember(ctx, execution, entity.ExecutionEvent{
			ExecutionID: execution.ID,
			Kind:        entity.ExecutionEventPhase,
			Actor:       runnerActor(runner),
			Reason:      proposedNote(plan),
			SourceID:    message.ID,
			OccurredAt:  occurred,
		}); err != nil {
			return err
		}

		postgres.AfterCommit(ctx, func(ctx context.Context) {
			s.publish(ctx, entity.EventExecutionPlan, execution)
		})

		return nil
	})
}

func (s *executionsService) planWaiting(ctx context.Context, execution entity.Execution) error {
	if execution.Stage != entity.StagePlanning {
		return entity.ErrExecutionNotPlanning
	}

	plans, err := s.plans.ListByExecution(ctx, execution.ID)
	if err != nil {
		return err
	}

	latest, ok := entity.LatestPlan(plans)
	if !ok || !latest.Undecided() {
		return entity.ErrExecutionPlanMissing
	}

	return nil
}

func (s *executionsService) ApprovePlan(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	revision int,
) (entity.Execution, error) {
	decision, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionManage)
	if err != nil {
		return entity.Execution{}, err
	}

	if err := s.deciding(ctx, decision, execution); err != nil {
		return entity.Execution{}, err
	}

	var (
		approved entity.Execution
		plan     entity.ExecutionPlan
	)

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		plan, err = s.plans.Approve(ctx, repository.PlanDecision{
			ExecutionID: execution.ID,
			Revision:    revision,
			AccountID:   decision.Actor.AccountID,
			At:          time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		approved, err = s.advance(ctx, execution, move{
			to:     entity.ExecutionQueuedForResume,
			stage:  entity.StageImplementation,
			reason: approvedNote(plan),
			actor:  entity.ExecutionActorOf(decision.Actor),
		})
		if err != nil {
			return err
		}

		postgres.AfterCommit(ctx, func(ctx context.Context) {
			s.publish(ctx, entity.EventExecutionPlan, approved)
		})

		return nil
	})
	if err != nil {
		return entity.Execution{}, err
	}

	if approved.RunnerID != uuid.Nil {
		if err := s.tell(ctx, approved, entity.ChannelExecutionResume, channelv1.Instruction{
			Reason:      channelv1.ResumePlanApproved,
			Stage:       entity.StageImplementation,
			Instruction: plan.Body,
		}); err != nil {
			return entity.Execution{}, err
		}
	}

	s.record(ctx, entity.AuditExecutionPlanned, approved)

	return approved, nil
}

func (s *executionsService) RevisePlan(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	revision int,
	feedback string,
) (entity.Execution, error) {
	decision, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionManage)
	if err != nil {
		return entity.Execution{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateExecutionFeedback("feedback", feedback),
	); err != nil {
		return entity.Execution{}, err
	}

	if err := s.deciding(ctx, decision, execution); err != nil {
		return entity.Execution{}, err
	}

	instruction := strings.TrimSpace(feedback)

	var revising entity.Execution

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.plans.RequestRevision(ctx, repository.PlanDecision{
			ExecutionID: execution.ID,
			Revision:    revision,
			AccountID:   decision.Actor.AccountID,
			Feedback:    instruction,
			At:          time.Now().UTC(),
		}); err != nil {
			return err
		}

		revising, err = s.advance(ctx, execution, move{
			to:     entity.ExecutionQueuedForResume,
			stage:  entity.StagePlanning,
			reason: instruction,
			actor:  entity.ExecutionActorOf(decision.Actor),
		})
		if err != nil {
			return err
		}

		postgres.AfterCommit(ctx, func(ctx context.Context) {
			s.publish(ctx, entity.EventExecutionPlan, revising)
		})

		return nil
	})
	if err != nil {
		return entity.Execution{}, err
	}

	if revising.RunnerID != uuid.Nil {
		if err := s.tell(ctx, revising, entity.ChannelExecutionResume, channelv1.Instruction{
			Reason:      channelv1.ResumePlanRevision,
			Stage:       entity.StagePlanning,
			Instruction: instruction,
		}); err != nil {
			return entity.Execution{}, err
		}
	}

	s.record(ctx, entity.AuditExecutionReplanned, revising)

	return revising, nil
}

func (s *executionsService) deciding(
	ctx context.Context,
	decision entity.Decision,
	execution entity.Execution,
) error {
	if execution.State != entity.ExecutionAwaitingPlan {
		return entity.ErrExecutionNotPlanning
	}

	if acting := decision.Actor.AgentID; acting != nil && *acting == execution.AgentID {
		return entity.ErrExecutionSelfApproval
	}

	if err := s.authorised(ctx, decision, execution); err != nil {
		return err
	}

	return s.settled(ctx, execution)
}

func (s *executionsService) settled(ctx context.Context, execution entity.Execution) error {
	questions, err := s.questions.ListByExecution(ctx, execution.WorkspaceID, execution.ID)
	if err != nil {
		return err
	}

	if len(entity.BlockingQuestionsOpen(questions)) > 0 {
		return entity.ErrExecutionQuestionsOpen
	}

	return nil
}

func (s *executionsService) publish(
	ctx context.Context,
	kind entity.EventKind,
	execution entity.Execution,
) {
	payload, err := json.Marshal(execution)
	if err != nil {
		return
	}

	s.events.Publish(ctx, entity.Event{
		WorkspaceID: execution.WorkspaceID,
		Kind:        kind,
		TeamID:      execution.TeamID,
		SubjectID:   execution.IssueID,
		IssueID:     execution.IssueID,
		Payload:     payload,
	})
}

func proposedNote(plan entity.ExecutionPlan) string {
	return "proposed plan revision " + strconv.Itoa(plan.Revision)
}

func approvedNote(plan entity.ExecutionPlan) string {
	return "approved plan revision " + strconv.Itoa(plan.Revision)
}

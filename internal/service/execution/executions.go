package execution

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const sweepBatch = 200

type executionsService struct {
	executions repository.Execution
	changesets repository.ChangeSet
	runners    repository.Runner
	issues     repository.Issue
	states     repository.WorkflowState
	channels   repository.RunnerChannel
	writer     service.Issues
	events     service.Events
	authorizer service.Authorizer
	audit      service.Audit
	transactor repository.Transactor
}

func New(
	executions repository.Execution,
	changesets repository.ChangeSet,
	runners repository.Runner,
	issues repository.Issue,
	states repository.WorkflowState,
	channels repository.RunnerChannel,
	writer service.Issues,
	events service.Events,
	authorizer service.Authorizer,
	audit service.Audit,
	transactor repository.Transactor,
) service.Executions {
	return &executionsService{
		executions: executions,
		changesets: changesets,
		runners:    runners,
		issues:     issues,
		states:     states,
		channels:   channels,
		writer:     writer,
		events:     events,
		authorizer: authorizer,
		audit:      audit,
		transactor: transactor,
	}
}

func (s *executionsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *executionsService) visible(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	action entity.Action,
) (entity.Decision, entity.Execution, error) {
	decision, err := s.decide(ctx, workspaceID, action)
	if err != nil {
		return entity.Decision{}, entity.Execution{}, err
	}

	execution, err := s.executions.GetByID(ctx, executionID)
	if err != nil {
		return entity.Decision{}, entity.Execution{}, err
	}

	if execution.WorkspaceID != workspaceID {
		return entity.Decision{}, entity.Execution{}, entity.ErrExecutionNotFound
	}

	if _, err := s.issues.GetVisible(
		ctx, workspaceID, execution.IssueID, decision.Scope,
	); err != nil {
		if errors.Is(err, entity.ErrIssueNotFound) {
			return entity.Decision{}, entity.Execution{}, entity.ErrExecutionNotFound
		}

		return entity.Decision{}, entity.Execution{}, err
	}

	return decision, execution, nil
}

func (s *executionsService) Visible(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) (entity.Execution, error) {
	_, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionRead)

	return execution, err
}

func (s *executionsService) Get(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) (service.ExecutionDetail, error) {
	_, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionRead)
	if err != nil {
		return service.ExecutionDetail{}, err
	}

	timeline, err := s.executions.ListEvents(ctx, execution.ID, entity.ExecutionTimelinePage{
		Limit: entity.ExecutionTimelinePreview,
	})
	if err != nil {
		return service.ExecutionDetail{}, err
	}

	changeset, err := s.changesets.Get(ctx, execution.ID)
	if err != nil {
		return service.ExecutionDetail{}, err
	}

	return service.ExecutionDetail{
		Execution: execution,
		Timeline:  timeline,
		ChangeSet: changeset,
	}, nil
}

func (s *executionsService) ListByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Execution, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return nil, err
	}

	return s.executions.ListByIssue(ctx, workspaceID, issue.ID)
}

func (s *executionsService) Timeline(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	page entity.ExecutionTimelinePage,
) ([]entity.ExecutionEvent, error) {
	_, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	return s.executions.ListEvents(ctx, execution.ID, page)
}

func (s *executionsService) Cancel(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID, reason string,
) (entity.Execution, error) {
	decision, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionManage)
	if err != nil {
		return entity.Execution{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateExecutionReason("reason", reason),
	); err != nil {
		return entity.Execution{}, err
	}

	if execution.Finished() {
		return entity.Execution{}, entity.ErrExecutionFinished
	}

	cancelled, err := s.advance(ctx, execution, move{
		to:     entity.ExecutionCancelled,
		reason: strings.TrimSpace(reason),
		actor:  entity.ExecutionActorOf(decision.Actor),
	})
	if err != nil {
		return entity.Execution{}, err
	}

	if cancelled.RunnerID != uuid.Nil {
		if err := s.tell(ctx, cancelled, entity.ChannelExecutionCancel, channelv1.Cancellation{
			Reason: cancelled.Reason,
		}); err != nil {
			return entity.Execution{}, err
		}
	}

	s.record(ctx, entity.AuditExecutionCancelled, cancelled)

	return cancelled, nil
}

func (s *executionsService) Approve(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) (entity.Execution, error) {
	decision, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionManage)
	if err != nil {
		return entity.Execution{}, err
	}

	if execution.State != entity.ExecutionAwaitingReview {
		return entity.Execution{}, entity.ErrExecutionNotReviewable
	}

	if acting := decision.Actor.AgentID; acting != nil && *acting == execution.AgentID {
		return entity.Execution{}, entity.ErrExecutionSelfApproval
	}

	approved, err := s.advance(ctx, execution, move{
		to:    entity.ExecutionApproved,
		actor: entity.ExecutionActorOf(decision.Actor),
	})
	if err != nil {
		return entity.Execution{}, err
	}

	if approved.RunnerID != uuid.Nil {
		if err := s.tell(ctx, approved, entity.ChannelExecutionResume, channelv1.Instruction{
			Reason: channelv1.ResumeApproved,
		}); err != nil {
			return entity.Execution{}, err
		}
	}

	s.record(ctx, entity.AuditExecutionApproved, approved)

	return approved, nil
}

func (s *executionsService) Resume(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID, feedback string,
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

	if execution.State != entity.ExecutionAwaitingReview {
		return entity.Execution{}, entity.ErrExecutionNotReviewable
	}

	instruction := strings.TrimSpace(feedback)

	resumed, err := s.advance(ctx, execution, move{
		to:     entity.ExecutionQueuedForResume,
		reason: instruction,
		actor:  entity.ExecutionActorOf(decision.Actor),
	})
	if err != nil {
		return entity.Execution{}, err
	}

	if resumed.RunnerID != uuid.Nil {
		if err := s.tell(ctx, resumed, entity.ChannelExecutionResume, channelv1.Instruction{
			Reason:      channelv1.ResumeFeedback,
			Instruction: instruction,
		}); err != nil {
			return entity.Execution{}, err
		}
	}

	return resumed, nil
}

func (s *executionsService) Restart(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) (entity.Execution, error) {
	decision, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionManage)
	if err != nil {
		return entity.Execution{}, err
	}

	if !execution.Finished() {
		return entity.Execution{}, entity.ErrExecutionUnfinished
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, execution.IssueID, decision.Scope)
	if err != nil {
		return entity.Execution{}, err
	}

	runner, err := s.runnerFor(ctx, execution.AgentID, issue.TeamID)
	if err != nil {
		return entity.Execution{}, err
	}

	return s.open(ctx, opening{
		issue:        issue,
		delegationID: execution.DelegationID,
		agentID:      execution.AgentID,
		runner:       runner,
		params:       execution.Params,
		actor:        entity.ExecutionActorOf(decision.Actor),
	})
}

func (s *executionsService) record(
	ctx context.Context,
	action entity.AuditAction,
	execution entity.Execution,
) {
	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  execution.WorkspaceID,
		Action:       action,
		ResourceKind: string(entity.ResourceIssue),
		ResourceID:   execution.IssueID,
		ResourceName: execution.Reference(),
	})
}

func leaseUntil(now time.Time) *time.Time {
	expiry := now.Add(entity.ChannelLeaseTTL)

	return &expiry
}

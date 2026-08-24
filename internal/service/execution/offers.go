package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

type opening struct {
	issue        entity.Issue
	delegationID uuid.UUID
	agentID      uuid.UUID
	placed       placement
	params       entity.ExecutionParams
	actor        entity.ExecutionActor
}

type move struct {
	to             entity.ExecutionState
	reason         string
	actor          entity.ExecutionActor
	runnerID       uuid.UUID
	leaseExpiresAt *time.Time
	sourceID       string
	occurredAt     time.Time
}

func paramsOf(params entity.ExecutionParams) channelv1.Params {
	return channelv1.Params{
		Tool:         params.Tool,
		Model:        params.Model,
		Runtime:      string(params.Runtime),
		BaseRef:      string(params.BaseRef),
		IncludeDirty: params.IncludeDirty,
		Profile:      string(params.Profile),
	}
}

func offerOf(execution entity.Execution, issue entity.Issue, branch string) channelv1.Offer {
	return channelv1.Offer{
		ExecutionID: execution.ID,
		Reference:   execution.Reference(),
		Attempt:     execution.Attempt,
		WorkspaceID: execution.WorkspaceID.String(),
		Branch:      branch,
		Issue: channelv1.Issue{
			ID:          issue.ID.String(),
			Reference:   issue.Reference(),
			Title:       issue.Title,
			Description: issue.Description,
			Brief:       execution.Params.Brief,
		},
		Params: paramsOf(execution.Params),
	}
}

func (s *executionsService) branchOf(
	ctx context.Context,
	execution entity.Execution,
	issue entity.Issue,
) string {
	branch, err := s.source.BranchNameForAgent(ctx, issue, execution.AgentID)
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"norn could not name the branch for a run, so the machine will name it",
			"execution_id", execution.ID,
			"issue_id", issue.ID.String(),
			"error", err,
		)

		return ""
	}

	return branch
}

func (s *executionsService) OnDelegated(
	ctx context.Context,
	issue entity.Issue,
	delegation entity.IssueDelegation,
) error {
	placed, err := s.route(ctx, delegation.AgentID, issue.TeamID, nil)
	if err != nil {
		return err
	}

	actor := entity.SystemExecutionActor()
	if acting, signedIn := identity.Actor(ctx); signedIn {
		actor = entity.ExecutionActorOf(acting)
	}

	params := delegation.Params.Asked()
	params.Brief = delegation.Brief

	_, err = s.open(ctx, opening{
		issue:        issue,
		delegationID: delegation.ID,
		agentID:      delegation.AgentID,
		placed:       placed,
		params:       params,
		actor:        actor,
	})

	return err
}

func (s *executionsService) Ready(ctx context.Context, runner entity.Runner) error {
	presence, err := s.channels.Presence(ctx, runner.ID)
	if err != nil {
		return err
	}

	if runner.Paused() || !presence.Available() {
		return nil
	}

	free, err := s.freeSlots(ctx, runner, presence)
	if err != nil {
		return err
	}

	if free <= 0 {
		return nil
	}

	queued, err := s.executions.ListQueuedByAgent(ctx, runner.AgentID, queuedBatch)
	if err != nil {
		return err
	}

	codebase, err := s.codebaseFor(ctx, runner)
	if err != nil {
		return err
	}

	for _, execution := range queued {
		offered := execution.RunnerID

		if offered != uuid.Nil && offered != runner.ID {
			continue
		}

		if !runner.Reaches(execution.TeamID) {
			continue
		}

		return s.hand(ctx, execution, placement{runner: runner, codebase: codebase, free: free})
	}

	return nil
}

func (s *executionsService) hand(
	ctx context.Context,
	execution entity.Execution,
	placed placement,
) error {
	issue, err := s.issues.GetVisible(
		ctx,
		execution.WorkspaceID,
		execution.IssueID,
		placed.runner.Scope(execution.WorkspaceID),
	)
	if err != nil {
		return err
	}

	bound, err := s.executions.Bind(ctx, execution.ID, repository.ExecutionBinding{
		RunnerID:   placed.runner.ID,
		CodebaseID: placed.codebase,
		At:         time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, entity.ErrExecutionTransition) {
			return nil
		}

		return err
	}

	if err := s.tell(
		ctx, bound, entity.ChannelExecutionOffer, offerOf(bound, issue, s.branchOf(ctx, bound, issue)),
	); err != nil {
		return err
	}

	s.broadcast(ctx, bound)

	return nil
}

func (s *executionsService) park(
	ctx context.Context,
	execution entity.Execution,
	waiting entity.ExecutionQueuedReason,
) error {
	parked, err := s.executions.Bind(ctx, execution.ID, repository.ExecutionBinding{
		QueuedReason: waiting,
		At:           time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, entity.ErrExecutionTransition) {
			return nil
		}

		return err
	}

	s.broadcast(ctx, parked)

	return nil
}

func (s *executionsService) open(
	ctx context.Context,
	request opening,
) (entity.Execution, error) {
	now := time.Now().UTC()

	var opened entity.Execution

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		attempt, err := s.executions.NextAttempt(ctx, request.issue.ID)
		if err != nil {
			return err
		}

		opened, err = s.executions.Create(ctx, repository.NewExecution{
			ID:           entity.NewExecutionID(ulid.Make().String()),
			WorkspaceID:  request.issue.WorkspaceID,
			IssueID:      request.issue.ID,
			DelegationID: request.delegationID,
			AgentID:      request.agentID,
			RunnerID:     request.placed.runner.ID,
			CodebaseID:   request.placed.codebase,
			Attempt:      attempt,
			QueuedReason: request.placed.waiting,
			Params:       request.params,
			QueuedAt:     now,
		})
		if err != nil {
			return err
		}

		if err := s.remember(ctx, opened, entity.ExecutionEvent{
			ExecutionID: opened.ID,
			Kind:        entity.ExecutionEventTransition,
			ToState:     entity.ExecutionQueued,
			Actor:       request.actor,
			Reason:      string(request.placed.waiting),
			OccurredAt:  now,
		}); err != nil {
			return err
		}

		postgres.AfterCommit(ctx, func(ctx context.Context) { s.broadcast(ctx, opened) })

		return nil
	})
	if err != nil {
		return entity.Execution{}, err
	}

	if !request.placed.found() {
		logging.From(ctx).InfoContext(
			ctx,
			"a delegated issue is waiting for a machine",
			slog.String("execution_id", opened.ID),
			slog.String("agent_id", request.agentID.String()),
			slog.String("waiting", string(request.placed.waiting)),
		)

		return opened, nil
	}

	if err := s.tell(
		ctx, opened, entity.ChannelExecutionOffer, offerOf(opened, request.issue, s.branchOf(ctx, opened, request.issue)),
	); err != nil {
		return entity.Execution{}, err
	}

	return opened, nil
}

func (s *executionsService) advance(
	ctx context.Context,
	execution entity.Execution,
	step move,
) (entity.Execution, error) {
	if !execution.State.CanTransitionTo(step.to) {
		return entity.Execution{}, entity.ErrExecutionTransition
	}

	now := time.Now().UTC()

	if step.occurredAt.IsZero() {
		step.occurredAt = now
	}

	lease := step.leaseExpiresAt
	if step.to.HoldsLease() && lease == nil {
		lease = leaseUntil(now)
	}

	if !step.to.HoldsLease() {
		lease = nil
	}

	var moved entity.Execution

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		var err error

		moved, err = s.executions.Move(ctx, execution.ID, repository.ExecutionMove{
			From:           execution.State,
			To:             step.to,
			Reason:         step.reason,
			RunnerID:       step.runnerID,
			LeaseExpiresAt: lease,
			At:             now,
		})
		if err != nil {
			return err
		}

		if err := s.remember(ctx, moved, entity.ExecutionEvent{
			ExecutionID: execution.ID,
			Kind:        entity.ExecutionEventTransition,
			FromState:   execution.State,
			ToState:     step.to,
			Actor:       step.actor,
			Reason:      step.reason,
			SourceID:    step.sourceID,
			OccurredAt:  step.occurredAt,
		}); err != nil {
			return err
		}

		postgres.AfterCommit(ctx, func(ctx context.Context) { s.broadcast(ctx, moved) })

		return nil
	})
	if err != nil {
		return entity.Execution{}, err
	}

	s.project(ctx, moved)

	return moved, nil
}

func (s *executionsService) remember(
	ctx context.Context,
	execution entity.Execution,
	event entity.ExecutionEvent,
) error {
	appended, err := s.executions.AppendEvent(ctx, event)
	if err != nil {
		if errors.Is(err, entity.ErrExecutionEventRecorded) {
			return nil
		}

		return err
	}

	postgres.AfterCommit(ctx, func(ctx context.Context) { s.announce(ctx, execution, appended) })

	return nil
}

func (s *executionsService) tell(
	ctx context.Context,
	execution entity.Execution,
	kind entity.ChannelMessageType,
	payload any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", kind, err)
	}

	message, err := entity.NewServerMessage(kind, execution.ID, body, time.Now().UTC())
	if err != nil {
		return err
	}

	if _, err := s.channels.Append(ctx, execution.RunnerID, message); err != nil {
		return err
	}

	return nil
}

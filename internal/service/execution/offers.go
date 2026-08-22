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
	runner       entity.Runner
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
		Tool:    params.Tool,
		Model:   params.Model,
		Runtime: string(params.Runtime),
	}
}

func offerOf(execution entity.Execution, issue entity.Issue) channelv1.Offer {
	return channelv1.Offer{
		ExecutionID: execution.ID,
		Reference:   execution.Reference(),
		Attempt:     execution.Attempt,
		WorkspaceID: execution.WorkspaceID.String(),
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

func (s *executionsService) OnDelegated(
	ctx context.Context,
	issue entity.Issue,
	delegation entity.IssueDelegation,
) error {
	runner, err := s.runnerFor(ctx, delegation.AgentID, issue.TeamID)
	if err != nil {
		if errors.Is(err, entity.ErrExecutionNoRunner) {
			logging.From(ctx).InfoContext(
				ctx,
				"an issue was delegated to an agent with no machine to run it on",
				slog.String("issue_id", issue.ID.String()),
				slog.String("agent_id", delegation.AgentID.String()),
			)

			return nil
		}

		return err
	}

	actor := entity.SystemExecutionActor()
	if acting, signedIn := identity.Actor(ctx); signedIn {
		actor = entity.ExecutionActorOf(acting)
	}

	_, err = s.open(ctx, opening{
		issue:        issue,
		delegationID: delegation.ID,
		agentID:      delegation.AgentID,
		runner:       runner,
		params:       entity.ExecutionParams{Brief: delegation.Brief},
		actor:        actor,
	})

	return err
}

func (s *executionsService) runnerFor(
	ctx context.Context,
	agentID, teamID uuid.UUID,
) (entity.Runner, error) {
	machines, err := s.runners.ListByAgentID(ctx, agentID)
	if err != nil {
		return entity.Runner{}, err
	}

	reaching := make([]entity.Runner, 0, len(machines))

	for _, machine := range machines {
		if machine.Reaches(teamID) {
			reaching = append(reaching, machine)
		}
	}

	if len(reaching) == 0 {
		return entity.Runner{}, entity.ErrExecutionNoRunner
	}

	for _, machine := range reaching {
		presence, err := s.channels.Presence(ctx, machine.ID)
		if err != nil {
			return entity.Runner{}, err
		}

		if presence.Live() {
			return machine, nil
		}
	}

	return reaching[0], nil
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
			RunnerID:     request.runner.ID,
			Attempt:      attempt,
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

	if err := s.tell(
		ctx, opened, entity.ChannelExecutionOffer, offerOf(opened, request.issue),
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

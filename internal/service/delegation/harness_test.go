package delegation_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	executionrepo "github.com/usenorn/norn/internal/repository/execution"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	delegationrepo "github.com/usenorn/norn/internal/repository/issuedelegation"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	delegationsvc "github.com/usenorn/norn/internal/service/delegation"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
)

type harness struct {
	delegations *delegationrepo.MockIssueDelegation
	issues      *issuerepo.MockIssue
	agents      *agentrepo.MockAgent
	runs        *executionrepo.MockExecution
	activity    *activityrepo.MockActivity
	emitter     *webhooksvc.MockWebhookEmitter
	executions  *executionsvc.MockExecutions
	authorizer  *authorizersvc.MockAuthorizer
	service     service.Delegations

	open        entity.IssueDelegation
	openFails   error
	run         entity.Execution
	runFails    error
	workspaceID uuid.UUID
	actorID     uuid.UUID
	actorKind   entity.ActorKind
	actorAgent  *uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		delegations: delegationrepo.NewMockIssueDelegation(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		agents:      agentrepo.NewMockAgent(ctrl),
		runs:        executionrepo.NewMockExecution(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		emitter:     webhooksvc.NewMockWebhookEmitter(ctrl),
		executions:  executionsvc.NewMockExecutions(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		openFails:   entity.ErrIssueDelegationNotFound,
		runFails:    entity.ErrExecutionNotFound,
		workspaceID: uuid.New(),
		actorID:     uuid.New(),
		actorKind:   entity.ActorKindUser,
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor: entity.Actor{
					Kind:      h.actorKind,
					AccountID: h.actorID,
					AgentID:   h.actorAgent,
				},
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	h.executions.EXPECT().
		OnDelegated(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	h.delegations.EXPECT().
		Open(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID) (entity.IssueDelegation, error) {
			return h.open, h.openFails
		}).
		AnyTimes()

	h.runs.EXPECT().
		LiveByDelegation(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID) (entity.Execution, error) {
			return h.run, h.runFails
		}).
		AnyTimes()

	h.service = delegationsvc.New(
		h.delegations, h.issues, h.agents, h.runs, h.activity, h.emitter, h.executions,
		h.authorizer, transactor,
	)

	return h
}

func (h *harness) expectIssue(issue entity.Issue) {
	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, issue.ID, gomock.Any()).
		Return(issue, nil).
		AnyTimes()
}

func (h *harness) expectAgent(agent entity.Agent) {
	h.agents.EXPECT().
		GetByAccountID(gomock.Any(), agent.AccountID).
		Return(agent, nil).
		AnyTimes()
}

func (h *harness) alreadyHeld(issue entity.Issue, agent entity.Agent) entity.IssueDelegation {
	held := entity.IssueDelegation{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		IssueID:     issue.ID,
		AgentID:     agent.ID,
		AgentName:   agent.Name,
		DelegatedAt: time.Now().UTC().Add(-time.Hour),
	}

	h.open, h.openFails = held, nil

	return held
}

func (h *harness) ranAs(
	held entity.IssueDelegation,
	state entity.ExecutionState,
	leaseExpiresAt *time.Time,
) {
	h.run = entity.Execution{
		ID:             "exec-01EARLIER",
		WorkspaceID:    h.workspaceID,
		IssueID:        held.IssueID,
		DelegationID:   held.ID,
		AgentID:        held.AgentID,
		Attempt:        1,
		State:          state,
		LeaseExpiresAt: leaseExpiresAt,
	}
	h.runFails = nil
}

func (h *harness) issue() entity.Issue {
	return entity.Issue{
		ID:                uuid.New(),
		WorkspaceID:       h.workspaceID,
		TeamID:            uuid.New(),
		AssigneeAccountID: uuid.New(),
		Version:           3,
		Status:            entity.IssueStatusActive,
		Priority:          entity.IssuePriorityNone,
		State:             entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryActive},
	}
}

func (h *harness) agent() entity.Agent {
	return entity.Agent{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AccountID:   uuid.New(),
		Name:        "opsy",
		Status:      entity.AgentStatusActive,
	}
}

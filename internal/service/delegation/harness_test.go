package delegation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
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
	activity    *activityrepo.MockActivity
	emitter     *webhooksvc.MockWebhookEmitter
	executions  *executionsvc.MockExecutions
	authorizer  *authorizersvc.MockAuthorizer
	service     service.Delegations

	workspaceID uuid.UUID
	actorID     uuid.UUID
	actorKind   entity.ActorKind
	actorAgent  *uuid.UUID
	actorOwner  uuid.UUID
	actorRole   entity.MembershipRole
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		delegations: delegationrepo.NewMockIssueDelegation(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		agents:      agentrepo.NewMockAgent(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		emitter:     webhooksvc.NewMockWebhookEmitter(ctrl),
		executions:  executionsvc.NewMockExecutions(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID: uuid.New(),
		actorID:     uuid.New(),
		actorKind:   entity.ActorKindUser,
		actorRole:   entity.MembershipRoleMember,
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
					Kind:           h.actorKind,
					AccountID:      h.actorID,
					AgentID:        h.actorAgent,
					OwnerAccountID: h.actorOwner,
				},
				Role:  h.actorRole,
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	h.executions.EXPECT().
		OnDelegated(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	h.service = delegationsvc.New(
		h.delegations, h.issues, h.agents, h.activity, h.emitter, h.executions, h.authorizer,
		transactor,
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

func (h *harness) expectDelegation(issue entity.Issue, agent entity.Agent) {
	h.delegations.EXPECT().
		Delegate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, held entity.IssueDelegation) (entity.IssueDelegation, error) {
			held.AgentName = agent.Name
			held.AgentAccountID = agent.AccountID

			return held, nil
		})

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)
	h.emitter.EXPECT().Emit(gomock.Any(), gomock.Any()).Return(nil)
}

func (h *harness) issue() entity.Issue {
	return entity.Issue{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		TeamID:      uuid.New(),
		Version:     3,
		Status:      entity.IssueStatusActive,
		Priority:    entity.IssuePriorityNone,
		State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryActive},
	}
}

func (h *harness) agent() entity.Agent {
	return entity.Agent{
		ID:             uuid.New(),
		WorkspaceID:    h.workspaceID,
		AccountID:      uuid.New(),
		OwnerAccountID: h.actorID,
		Name:           "opsy",
		Status:         entity.AgentStatusActive,
	}
}

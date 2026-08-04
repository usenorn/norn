package workflowstate_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	workflowstatesvc "github.com/usenorn/norn/internal/service/workflowstate"
)

type harness struct {
	states     *workflowstaterepo.MockWorkflowState
	issues     *issuerepo.MockIssue
	teams      *teamrepo.MockTeam
	authorizer *authorizersvc.MockAuthorizer
	transactor *transactorrepo.MockTransactor
	service    service.WorkflowStates
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		states:     workflowstaterepo.NewMockWorkflowState(ctrl),
		issues:     issuerepo.NewMockIssue(ctrl),
		teams:      teamrepo.NewMockTeam(ctrl),
		authorizer: authorizersvc.NewMockAuthorizer(ctrl),
		transactor: transactorrepo.NewMockTransactor(ctrl),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = workflowstatesvc.New(h.states, h.issues, h.teams, h.authorizer, h.transactor)

	return h
}

func (h *harness) expectActorMayManage(workspaceID, teamID uuid.UUID) {
	h.expectDecision(workspaceID, teamID, entity.TeamStatusActive)
}

func (h *harness) expectDecision(workspaceID, teamID uuid.UUID, status entity.TeamStatus) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
			Role:  entity.MembershipRoleAdmin,
			Scope: entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true},
		}, nil)

	h.teams.EXPECT().
		GetByID(gomock.Any(), teamID).
		Return(entity.Team{ID: teamID, WorkspaceID: workspaceID, Key: "MOB", Name: "Mobile", Status: status}, nil)
}

func (h *harness) expectLocked(teamID uuid.UUID, states []entity.WorkflowState) {
	h.states.EXPECT().LockByTeamID(gomock.Any(), teamID).Return(states, nil)
}

func seededStates(workspaceID, teamID uuid.UUID) []entity.WorkflowState {
	states := entity.DefaultWorkflowStates(workspaceID, teamID)

	for i := range states {
		states[i].ID = uuid.New()
	}

	return states
}

func byName(states []entity.WorkflowState, name string) entity.WorkflowState {
	for _, state := range states {
		if state.Name == name {
			return state
		}
	}

	return entity.WorkflowState{}
}

func idsOf(states []entity.WorkflowState) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(states))

	for _, state := range states {
		ids = append(ids, state.ID)
	}

	return ids
}

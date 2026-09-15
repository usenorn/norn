package issue_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) raising(workspaceID, teamID uuid.UUID) *entity.Issue {
	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	h.states.EXPECT().
		DefaultForTeam(gomock.Any(), teamID).
		Return(entity.WorkflowState{ID: uuid.New(), TeamID: teamID, IsDefault: true}, nil)
	h.teams.EXPECT().
		ListByWorkspaceMember(gomock.Any(), workspaceID, gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)

	raised := &entity.Issue{}

	h.issues.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, issue entity.Issue) (entity.Issue, error) {
			*raised = issue

			return issue, nil
		})

	return raised
}

func (h *harness) assignableTo(workspaceID, personID uuid.UUID) {
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, personID).
		Return(entity.Membership{WorkspaceID: workspaceID, AccountID: personID}, nil)
	h.accounts.EXPECT().
		GetByID(gomock.Any(), personID).
		Return(entity.Account{ID: personID, Kind: entity.AccountKindPerson}, nil)
}

func TestAnIssueAnAgentRaisesForSomebodyIsNotLeftWaitingInTriage(t *testing.T) {
	h := newHarness(t)
	h.actingAs(entity.ActorKindAgent)
	h.triaging(entity.TriageSettings{RouteAgents: true})

	workspaceID, teamID, personID := uuid.New(), uuid.New(), uuid.New()

	raised := h.raising(workspaceID, teamID)
	h.assignableTo(workspaceID, personID)

	if _, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID:       workspaceID,
		TeamID:            teamID,
		Title:             "Storage limits",
		AssigneeAccountID: personID,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if raised.TriageState.Waiting() {
		t.Fatalf(
			"an issue an agent raised for %v was left waiting in triage. Every issue listing leaves "+
				"out what is waiting, so the team's list, a filter by that assignee and their My "+
				"tasks would all miss work that already has their name on it, and only a direct "+
				"link would reach it.",
			personID,
		)
	}
}

func TestAnIssueAnAgentRaisesForNobodyStillWaitsInTriage(t *testing.T) {
	h := newHarness(t)
	h.actingAs(entity.ActorKindAgent)
	h.triaging(entity.TriageSettings{RouteAgents: true})

	workspaceID, teamID := uuid.New(), uuid.New()

	raised := h.raising(workspaceID, teamID)

	if _, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "Something an agent noticed",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !raised.TriageState.Waiting() || raised.TriageSource != entity.TriageSourceAgent {
		t.Fatalf(
			"an issue an agent raised for nobody was filed as %q from %q, want it waiting in triage "+
				"as an agent's. A team that asked to triage what agents raise still has to see "+
				"unowned work before it joins the backlog.",
			raised.TriageState, raised.TriageSource,
		)
	}
}

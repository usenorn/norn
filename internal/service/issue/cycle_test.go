package issue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) expectDecision(workspaceID uuid.UUID, teamIDs ...uuid.UUID) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
			Workspace: entity.Workspace{ID: workspaceID, Timezone: "UTC"},
			Scope:     entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: teamIDs},
		}, nil)
}

func runningCycle(workspaceID, teamID uuid.UUID) entity.Cycle {
	now := time.Now().UTC()

	return entity.Cycle{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Number:      4,
		StartsOn:    entity.Today(now.AddDate(0, 0, -3), "UTC"),
		EndsOn:      entity.Today(now.AddDate(0, 0, 3), "UTC"),
	}
}

func TestChangingSomethingOtherThanTheCycleLeavesTheScopeLedgerAlone(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()
	cycle := runningCycle(workspaceID, teamID)
	stateID := uuid.New()

	h.expectDecision(workspaceID, teamID)

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:          issueID,
			WorkspaceID: workspaceID,
			TeamID:      teamID,
			Version:     1,
			CycleID:     cycle.ID,
			CycleNumber: cycle.Number,
			State:       entity.IssueState{ID: uuid.New(), Name: "Ready", Category: entity.StateCategoryNotStarted},
		}, nil)

	h.states.EXPECT().ListByTeamID(gomock.Any(), teamID).Return([]entity.WorkflowState{{
		ID:       stateID,
		TeamID:   teamID,
		Name:     "Shipped",
		Category: entity.StateCategoryComplete,
	}}, nil)

	h.issues.EXPECT().ListChildren(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(nil, nil)
	h.issues.EXPECT().Update(gomock.Any(), issueID, 1, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(entity.Issue{}, nil)

	h.scope.EXPECT().Record(gomock.Any(), gomock.Any()).Times(0)

	if _, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &stateID,
	}); err != nil {
		t.Fatalf(
			"Update: %v — moving an issue between states must not touch the cycle's scope ledger; "+
				"otherwise finishing work reads as taking it out of the cycle",
			err,
		)
	}
}

func TestJoiningACycleAfterItStartedIsRecordedAsAddedScope(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()
	cycle := runningCycle(workspaceID, teamID)

	h.expectDecision(workspaceID, teamID)

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:          issueID,
			WorkspaceID: workspaceID,
			TeamID:      teamID,
			Version:     1,
			State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryNotStarted},
		}, nil)

	h.cycles.EXPECT().
		GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).
		Return(cycle, nil)

	h.issues.EXPECT().Update(gomock.Any(), issueID, 1, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(entity.Issue{}, nil)

	var recorded entity.CycleScopeChange

	h.scope.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, change entity.CycleScopeChange) error {
			recorded = change

			return nil
		})

	if _, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		CycleID:         &cycle.ID,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if recorded.Change != entity.CycleScopeChangeAdded || recorded.IssueID != issueID {
		t.Fatalf(
			"ledger recorded %q for issue %v, want an %q entry for %v so the cycle can show what "+
				"arrived after it was planned",
			recorded.Change, recorded.IssueID, entity.CycleScopeChangeAdded, issueID,
		)
	}
}

func TestJoiningACycleBeforeItStartsCountsAsOriginalScope(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	cycle := entity.Cycle{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Number:      5,
		StartsOn:    entity.Today(now.AddDate(0, 0, 7), "UTC"),
		EndsOn:      entity.Today(now.AddDate(0, 0, 20), "UTC"),
	}

	h.expectDecision(workspaceID, teamID)

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:          issueID,
			WorkspaceID: workspaceID,
			TeamID:      teamID,
			Version:     1,
			State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryNotStarted},
		}, nil)

	h.cycles.EXPECT().GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).Return(cycle, nil)
	h.issues.EXPECT().Update(gomock.Any(), issueID, 1, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(entity.Issue{}, nil)

	h.scope.EXPECT().Record(gomock.Any(), gomock.Any()).Times(0)

	if _, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		CycleID:         &cycle.ID,
	}); err != nil {
		t.Fatalf(
			"Update: %v — planning work into a cycle before it starts is the original scope, "+
				"not a change to it",
			err,
		)
	}
}

func TestAnIssueCannotJoinAnotherTeamsCycle(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()
	cycle := runningCycle(workspaceID, uuid.New())

	h.expectDecision(workspaceID, teamID)

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:          issueID,
			WorkspaceID: workspaceID,
			TeamID:      teamID,
			Version:     1,
			State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryNotStarted},
		}, nil)

	h.cycles.EXPECT().GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).Return(cycle, nil)

	_, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		CycleID:         &cycle.ID,
	})

	if !errors.Is(err, entity.ErrCycleTeamMismatch) {
		t.Fatalf("Update returned %v, want ErrCycleTeamMismatch", err)
	}
}

func TestAClosedCycleRefusesToTakeAnyMoreWork(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()
	closedAt := time.Now().UTC()
	cycle := runningCycle(workspaceID, teamID)
	cycle.ClosedAt = &closedAt

	h.expectDecision(workspaceID, teamID)

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:          issueID,
			WorkspaceID: workspaceID,
			TeamID:      teamID,
			Version:     1,
			State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryNotStarted},
		}, nil)

	h.cycles.EXPECT().GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).Return(cycle, nil)

	_, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		CycleID:         &cycle.ID,
	})

	if !errors.Is(err, entity.ErrCycleClosed) {
		t.Fatalf(
			"Update returned %v, want ErrCycleClosed; what a closed cycle contained is history "+
				"and must not change afterwards",
			err,
		)
	}
}

func TestMovingAnIssueToAnotherTeamTakesItOutOfItsCycle(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, destinationID, issueID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	cycle := runningCycle(workspaceID, teamID)
	stateID := uuid.New()

	h.expectDecision(workspaceID, teamID, destinationID)

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:          issueID,
			WorkspaceID: workspaceID,
			TeamID:      teamID,
			Version:     1,
			CycleID:     cycle.ID,
			State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryNotStarted},
		}, nil)

	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{ID: issueID, TeamID: teamID}, nil).
		AnyTimes()

	h.states.EXPECT().ListByTeamID(gomock.Any(), destinationID).Return([]entity.WorkflowState{{
		ID:       stateID,
		TeamID:   destinationID,
		Name:     "Todo",
		Category: entity.StateCategoryNotStarted,
	}}, nil)

	h.issues.EXPECT().
		MoveToTeam(gomock.Any(), issueID, 1, destinationID, stateID, gomock.Any(), gomock.Any()).
		Return(nil)

	h.cycles.EXPECT().GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).Return(cycle, nil)

	var recorded entity.CycleScopeChange

	h.scope.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, change entity.CycleScopeChange) error {
			recorded = change

			return nil
		})

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	if _, err := h.service.MoveToTeam(context.Background(), workspaceID, issueID, service.MoveIssueInput{
		ExpectedVersion:      1,
		TeamID:               destinationID,
		AcknowledgeLabelLoss: true,
	}); err != nil {
		t.Fatalf("MoveToTeam: %v", err)
	}

	if recorded.Change != entity.CycleScopeChangeRemoved || recorded.CycleID != cycle.ID {
		t.Fatalf(
			"ledger recorded %q on cycle %v, want %q on %v; a cycle belongs to one team, so "+
				"leaving the team leaves the cycle",
			recorded.Change, recorded.CycleID, entity.CycleScopeChangeRemoved, cycle.ID,
		)
	}
}

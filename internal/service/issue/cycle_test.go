package issue_test

import (
	"context"
	"encoding/json"
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

func closedCycle(workspaceID, teamID uuid.UUID) entity.Cycle {
	closedAt := time.Date(2023, time.March, 20, 17, 0, 0, 0, time.UTC)

	return entity.Cycle{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Number:      1,
		StartsOn:    "2023-03-06",
		EndsOn:      "2023-03-19",
		ClosedAt:    &closedAt,
	}
}

func sourceOrigin() entity.ImportOrigin {
	created := time.Date(2023, time.March, 7, 9, 15, 0, 0, time.UTC)

	return entity.NewImportOrigin(created, created.AddDate(0, 0, 4), uuid.New())
}

func (h *harness) expectRaising(workspaceID, teamID uuid.UUID) *entity.Issue {
	h.expectDecision(workspaceID, teamID)

	h.states.EXPECT().
		DefaultForTeam(gomock.Any(), teamID).
		Return(entity.WorkflowState{ID: uuid.New(), TeamID: teamID, IsDefault: true}, nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	captured := &entity.Issue{}

	h.issues.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, issue entity.Issue) (entity.Issue, error) {
			*captured = issue
			issue.ID = uuid.New()

			return issue, nil
		}).
		AnyTimes()

	return captured
}

func TestAnImportedIssueIsRaisedInsideTheClosedCycleItCameFrom(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID := uuid.New(), uuid.New()
	cycle := closedCycle(workspaceID, teamID)
	origin := sourceOrigin()

	captured := h.expectRaising(workspaceID, teamID)
	h.cycles.EXPECT().GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).Return(cycle, nil)

	if _, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "Offline queue drops edits on reconnect",
		CycleID:     cycle.ID,
		Origin:      &origin,
	}); err != nil {
		t.Fatalf(
			"Create: %v — an import carries a team's finished cycles, and finished is the only "+
				"state a historical cycle can be in; refusing this imports cycles with nothing in them",
			err,
		)
	}

	if captured.CycleID != cycle.ID {
		t.Fatalf(
			"the issue reached the store in cycle %v, want %v",
			captured.CycleID, cycle.ID,
		)
	}
}

func TestAnOriginFilledInFromOutsideDoesNotOpenAClosedCycle(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID := uuid.New(), uuid.New()
	cycle := closedCycle(workspaceID, teamID)

	h.expectRaising(workspaceID, teamID)
	h.cycles.EXPECT().GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).Return(cycle, nil)

	var decoded entity.ImportOrigin

	if err := json.Unmarshal(
		[]byte(`{"CreatedAt":"2019-04-02T09:15:00Z","AuthorAccountID":"`+uuid.New().String()+`"}`),
		&decoded,
	); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "Offline queue drops edits on reconnect",
		CycleID:     cycle.ID,
		Origin:      &decoded,
	})

	if !errors.Is(err, entity.ErrCycleClosed) {
		t.Fatalf(
			"Create returned %v, want ErrCycleClosed. The origin here was decoded from JSON, so "+
				"it is non-nil and inert — testing the pointer for presence rather than the value "+
				"for attribution would hand this concession to anybody who named the field, and a "+
				"closed cycle is history the product must not add to.",
			err,
		)
	}
}

func TestRaisingAnIssueInAClosedCycleWithoutAnImportIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID := uuid.New(), uuid.New()
	cycle := closedCycle(workspaceID, teamID)

	h.expectRaising(workspaceID, teamID)
	h.cycles.EXPECT().GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).Return(cycle, nil)

	_, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "An issue",
		CycleID:     cycle.ID,
	})

	if !errors.Is(err, entity.ErrCycleClosed) {
		t.Fatalf(
			"Create returned %v, want ErrCycleClosed; the concession belongs to an import alone, "+
				"and what a closed cycle contained is history the product must not add to",
			err,
		)
	}
}

func TestARunningCycleTakesANewIssueWhetherOrNotItWasImported(t *testing.T) {
	origin := sourceOrigin()

	for name, carried := range map[string]*entity.ImportOrigin{
		"raised by hand": nil,
		"imported":       &origin,
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			workspaceID, teamID := uuid.New(), uuid.New()
			cycle := runningCycle(workspaceID, teamID)

			captured := h.expectRaising(workspaceID, teamID)
			h.cycles.EXPECT().
				GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).
				Return(cycle, nil)

			if _, err := h.service.Create(context.Background(), service.CreateIssueInput{
				WorkspaceID: workspaceID,
				TeamID:      teamID,
				Title:       "An issue",
				CycleID:     cycle.ID,
				Origin:      carried,
			}); err != nil {
				t.Fatalf("Create (%s): %v — an open cycle takes work from either path", name, err)
			}

			if captured.CycleID != cycle.ID {
				t.Fatalf("the issue reached the store in cycle %v, want %v", captured.CycleID, cycle.ID)
			}
		})
	}
}

func TestAnIssueCannotBeRaisedInAnotherTeamsCycle(t *testing.T) {
	origin := sourceOrigin()

	for name, carried := range map[string]*entity.ImportOrigin{
		"raised by hand": nil,
		"imported":       &origin,
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			workspaceID, teamID := uuid.New(), uuid.New()
			cycle := runningCycle(workspaceID, uuid.New())

			h.expectRaising(workspaceID, teamID)
			h.cycles.EXPECT().
				GetVisible(gomock.Any(), workspaceID, cycle.ID, gomock.Any()).
				Return(cycle, nil)

			_, err := h.service.Create(context.Background(), service.CreateIssueInput{
				WorkspaceID: workspaceID,
				TeamID:      teamID,
				Title:       "An issue",
				CycleID:     cycle.ID,
				Origin:      carried,
			})

			if !errors.Is(err, entity.ErrCycleTeamMismatch) {
				t.Fatalf(
					"Create (%s) returned %v, want ErrCycleTeamMismatch; a cycle belongs to one "+
						"team, and an import must not smuggle work across that line",
					name, err,
				)
			}
		})
	}
}

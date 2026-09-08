package workflowstate_test

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

func TestRemovingAnOccupiedStateRequiresAndAppliesATarget(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	states := seededStates(workspaceID, teamID)

	removed := byName(states, "In review")
	replacement := byName(states, "In progress")

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, states)

	var reassignedFrom, reassignedTo uuid.UUID

	h.issues.EXPECT().
		ReassignState(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, from, to uuid.UUID) error {
			reassignedFrom, reassignedTo = from, to

			return nil
		})
	h.states.EXPECT().Delete(gomock.Any(), removed.ID).Return(nil)
	h.states.EXPECT().Reposition(gomock.Any(), teamID, gomock.Any()).Return(nil)

	if err := h.service.Remove(context.Background(), workspaceID, teamID, removed.ID, replacement.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if reassignedFrom != removed.ID || reassignedTo != replacement.ID {
		t.Fatalf("reassigned %v -> %v, want %v -> %v", reassignedFrom, reassignedTo, removed.ID, replacement.ID)
	}
}

func TestRemovingAStateWithAnUnknownTargetTouchesNothing(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	states := seededStates(workspaceID, teamID)

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, states)

	err := h.service.Remove(context.Background(), workspaceID, teamID, byName(states, "In review").ID, uuid.New())

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Remove error = %v, want a ValidationError", err)
	}
}

func TestATeamCannotBeLeftWithoutADefaultState(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	states := seededStates(workspaceID, teamID)

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, states)

	err := h.service.Remove(
		context.Background(),
		workspaceID,
		teamID,
		byName(states, "Todo").ID,
		byName(states, "Backlog").ID,
	)

	if !errors.Is(err, entity.ErrWorkflowStateIsDefault) {
		t.Fatalf("Remove error = %v, want ErrWorkflowStateIsDefault", err)
	}
}

func TestATeamCannotBeLeftWithoutACompletionState(t *testing.T) {
	workspaceID := uuid.New()
	teamID := uuid.New()

	t.Run("by removal", func(t *testing.T) {
		h := newHarness(t)
		states := seededStates(workspaceID, teamID)

		h.expectActorMayManage(workspaceID, teamID)
		h.expectLocked(teamID, states)

		err := h.service.Remove(
			context.Background(),
			workspaceID,
			teamID,
			byName(states, "Done").ID,
			byName(states, "Canceled").ID,
		)

		if !errors.Is(err, entity.ErrWorkflowStateIsCompletion) {
			t.Fatalf("Remove error = %v, want ErrWorkflowStateIsCompletion", err)
		}
	})

	t.Run("by recategorizing", func(t *testing.T) {
		h := newHarness(t)
		states := seededStates(workspaceID, teamID)

		h.expectActorMayManage(workspaceID, teamID)
		h.expectLocked(teamID, states)

		category := entity.StateCategoryActive

		_, err := h.service.Update(
			context.Background(),
			workspaceID,
			teamID,
			byName(states, "Done").ID,
			service.UpdateWorkflowStateInput{Category: &category},
		)

		if !errors.Is(err, entity.ErrWorkflowStateIsCompletion) {
			t.Fatalf("Update error = %v, want ErrWorkflowStateIsCompletion", err)
		}
	})
}

func TestACategoryCannotBeEmptied(t *testing.T) {
	workspaceID := uuid.New()
	teamID := uuid.New()

	t.Run("removing the only state in a category", func(t *testing.T) {
		h := newHarness(t)
		states := seededStates(workspaceID, teamID)

		h.expectActorMayManage(workspaceID, teamID)
		h.expectLocked(teamID, states)

		err := h.service.Remove(
			context.Background(),
			workspaceID,
			teamID,
			byName(states, "Canceled").ID,
			byName(states, "Backlog").ID,
		)

		if !errors.Is(err, entity.ErrWorkflowStateLastInCategory) {
			t.Fatalf("Remove error = %v, want ErrWorkflowStateLastInCategory", err)
		}
	})

	t.Run("recategorizing the only state in a category", func(t *testing.T) {
		h := newHarness(t)
		states := seededStates(workspaceID, teamID)

		h.expectActorMayManage(workspaceID, teamID)
		h.expectLocked(teamID, states)

		category := entity.StateCategoryActive

		_, err := h.service.Update(
			context.Background(),
			workspaceID,
			teamID,
			byName(states, "Canceled").ID,
			service.UpdateWorkflowStateInput{Category: &category},
		)

		if !errors.Is(err, entity.ErrWorkflowStateLastInCategory) {
			t.Fatalf("Update error = %v, want ErrWorkflowStateLastInCategory", err)
		}
	})

	t.Run("removing one of a pair succeeds", func(t *testing.T) {
		h := newHarness(t)
		states := seededStates(workspaceID, teamID)

		removed := byName(states, "In review")

		h.expectActorMayManage(workspaceID, teamID)
		h.expectLocked(teamID, states)
		h.issues.EXPECT().ReassignState(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		h.states.EXPECT().Delete(gomock.Any(), removed.ID).Return(nil)
		h.states.EXPECT().Reposition(gomock.Any(), teamID, gomock.Any()).Return(nil)

		if err := h.service.Remove(
			context.Background(),
			workspaceID,
			teamID,
			removed.ID,
			byName(states, "In progress").ID,
		); err != nil {
			t.Fatalf("removing one of two active states must succeed, got %v", err)
		}
	})
}

func TestReorderProducesAStableTotalOrder(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	states := seededStates(workspaceID, teamID)

	shuffled := []uuid.UUID{
		byName(states, "Done").ID,
		byName(states, "Backlog").ID,
		byName(states, "Canceled").ID,
		byName(states, "In progress").ID,
		byName(states, "Todo").ID,
		byName(states, "In review").ID,
	}

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, states)

	var captured []uuid.UUID

	h.states.EXPECT().
		Reposition(gomock.Any(), teamID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, ids []uuid.UUID) error {
			captured = ids

			return nil
		})

	reordered, err := h.service.Reorder(context.Background(), workspaceID, teamID, shuffled)
	if err != nil {
		t.Fatalf("Reorder: %v", err)
	}

	for i, id := range shuffled {
		if captured[i] != id {
			t.Fatalf("the store was given a different order at %d", i)
		}

		if reordered[i].ID != id || reordered[i].Position != i+1 {
			t.Fatalf("state %d is %v at position %d, want %v at %d", i, reordered[i].ID, reordered[i].Position, id, i+1)
		}
	}
}

func TestAnIncompleteOrDuplicatedOrderIsRefused(t *testing.T) {
	workspaceID := uuid.New()
	teamID := uuid.New()

	cases := map[string]func([]entity.WorkflowState) []uuid.UUID{
		"missing a state": func(states []entity.WorkflowState) []uuid.UUID {
			return idsOf(states)[:len(states)-1]
		},
		"duplicating a state": func(states []entity.WorkflowState) []uuid.UUID {
			ids := idsOf(states)
			ids[0] = ids[1]

			return ids
		},
		"naming an unknown state": func(states []entity.WorkflowState) []uuid.UUID {
			ids := idsOf(states)
			ids[0] = uuid.New()

			return ids
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			states := seededStates(workspaceID, teamID)

			h.expectActorMayManage(workspaceID, teamID)
			h.expectLocked(teamID, states)

			_, err := h.service.Reorder(context.Background(), workspaceID, teamID, mutate(states))

			var validation entity.ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("Reorder error = %v, want a ValidationError", err)
			}
		})
	}
}

func TestTheCompletionStateMustBeInTheCompleteCategory(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	states := seededStates(workspaceID, teamID)

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, states)

	_, err := h.service.SetCompletion(context.Background(), workspaceID, teamID, byName(states, "Todo").ID)

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("SetCompletion error = %v, want a ValidationError", err)
	}
}

func TestMovingTheDefaultNamesItsSuccessor(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	states := seededStates(workspaceID, teamID)

	successor := byName(states, "Backlog")

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, states)
	h.states.EXPECT().SetDefault(gomock.Any(), teamID, successor.ID).Return(nil)

	flagged, err := h.service.SetDefault(context.Background(), workspaceID, teamID, successor.ID)
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	defaults := make([]uuid.UUID, 0, 1)

	for _, state := range flagged {
		if state.IsDefault {
			defaults = append(defaults, state.ID)
		}
	}

	if len(defaults) != 1 || defaults[0] != successor.ID {
		t.Fatalf("states flagged default = %v, want exactly [%v]", defaults, successor.ID)
	}
}

func TestAStateOutsideTheActorsTeamScopeIsHiddenRatherThanForbidden(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
			Role:  entity.MembershipRoleMember,
			Scope: entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{uuid.New()}},
		}, nil)

	h.teams.EXPECT().
		GetByID(gomock.Any(), teamID).
		Return(entity.Team{ID: teamID, WorkspaceID: workspaceID, Status: entity.TeamStatusActive}, nil)

	_, err := h.service.List(context.Background(), workspaceID, teamID)

	if !errors.Is(err, entity.ErrTeamNotFound) {
		t.Fatalf("List error = %v, want ErrTeamNotFound", err)
	}

	if errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatal("a team outside the scope must not answer forbidden, which would confirm it exists")
	}
}

func TestAnArchivedTeamsStatesStayReadableButNotEditable(t *testing.T) {
	workspaceID := uuid.New()
	teamID := uuid.New()

	t.Run("readable", func(t *testing.T) {
		h := newHarness(t)

		h.expectDecision(workspaceID, teamID, entity.TeamStatusArchived)
		h.states.EXPECT().ListByTeamID(gomock.Any(), teamID).Return(nil, nil)

		if _, err := h.service.List(context.Background(), workspaceID, teamID); err != nil {
			t.Fatalf("an archived team's states must stay readable, got %v", err)
		}
	})

	t.Run("not editable", func(t *testing.T) {
		h := newHarness(t)

		h.expectDecision(workspaceID, teamID, entity.TeamStatusArchived)

		_, err := h.service.Create(context.Background(), service.CreateWorkflowStateInput{
			WorkspaceID: workspaceID,
			TeamID:      teamID,
			Name:        "Blocked",
			Category:    entity.StateCategoryActive,
		})

		if !errors.Is(err, entity.ErrTeamArchived) {
			t.Fatalf("Create error = %v, want ErrTeamArchived", err)
		}
	})
}

func TestANewStateLandsAtTheEndOfTheOrder(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	states := seededStates(workspaceID, teamID)

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, states)

	var captured entity.WorkflowState

	h.states.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, state entity.WorkflowState) (entity.WorkflowState, error) {
			captured = state

			return state, nil
		})

	if _, err := h.service.Create(context.Background(), service.CreateWorkflowStateInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Name:        "Blocked",
		Category:    entity.StateCategoryActive,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.Position != len(states)+1 {
		t.Fatalf("new state landed at position %d, want %d", captured.Position, len(states)+1)
	}

	if captured.IsDefault || captured.IsCompletion {
		t.Fatal("a newly created state must not claim the default or completion flag")
	}
}

func TestAnImportedStateReachesTheRepositoryWithTheDatesItsSourceRecorded(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, seededStates(workspaceID, teamID))

	var captured entity.WorkflowState

	h.states.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, state entity.WorkflowState) (entity.WorkflowState, error) {
			captured = state

			return state, nil
		})

	createdAt := time.Date(2019, time.April, 2, 9, 15, 0, 0, time.UTC)
	updatedAt := createdAt.Add(72 * time.Hour)
	origin := entity.NewImportOrigin(createdAt, updatedAt, uuid.New())

	if _, err := h.service.Create(context.Background(), service.CreateWorkflowStateInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Name:        "Blocked",
		Category:    entity.StateCategoryActive,
		Origin:      &origin,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.Origin == nil {
		t.Fatal("the origin stopped at the service, so the state would be dated the moment the import ran")
	}

	gotCreated, gotUpdated := captured.Origin.Stamp(time.Now().UTC())

	if !gotCreated.Equal(createdAt) || !gotUpdated.Equal(updatedAt) {
		t.Fatalf("stamp = (%v, %v), want (%v, %v)", gotCreated, gotUpdated, createdAt, updatedAt)
	}
}

func TestReclassifyingAStateLeavesEvidenceOnEveryIssueStandingInIt(t *testing.T) {
	workspaceID, teamID := uuid.New(), uuid.New()

	h := newHarness(t)
	states := seededStates(workspaceID, teamID)
	moving := byName(states, "In review")

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, states)

	waiting := entity.Issue{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Version:     4,
		State:       entity.IssueState{ID: moving.ID, Name: moving.Name, Category: moving.Category},
	}

	h.inState[moving.ID] = []entity.Issue{waiting}
	h.states.EXPECT().
		UpdateSettings(gomock.Any(), moving.ID, moving.Name, entity.StateCategoryComplete).
		Return(moving, nil)

	category := entity.StateCategoryComplete

	if _, err := h.service.Update(context.Background(), workspaceID, teamID, moving.ID, service.UpdateWorkflowStateInput{
		Category: &category,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(h.recorded) != 1 {
		t.Fatalf(
			"changing what kind of state %q is left %d entries in the ledger, want one for the "+
				"issue standing in it. Every issue in that state started counting as finished "+
				"work that instant, and a burndown read afterwards would otherwise show them as "+
				"having been finished all along.",
			moving.Name, len(h.recorded),
		)
	}

	entry := h.recorded[0]

	if entry.Kind != entity.ActivityKindStateReclassified {
		t.Errorf("the entry is a %q, want a reclassification: the issue did not move", entry.Kind)
	}

	if entry.FromCategory != moving.Category || entry.ToCategory != entity.StateCategoryComplete {
		t.Errorf(
			"the entry records %q to %q, want %q to %q",
			entry.FromCategory, entry.ToCategory, moving.Category, entity.StateCategoryComplete,
		)
	}

	if entry.FromState != moving.Name || entry.ToState != moving.Name {
		t.Errorf("the entry renames the state from %q to %q, but only its kind changed", entry.FromState, entry.ToState)
	}
}

func TestRemovingAStateRecordsWhereItsIssuesWent(t *testing.T) {
	workspaceID, teamID := uuid.New(), uuid.New()

	h := newHarness(t)
	states := seededStates(workspaceID, teamID)
	removed := byName(states, "In review")
	replacement := byName(states, "In progress")

	h.expectActorMayManage(workspaceID, teamID)
	h.expectLocked(teamID, states)

	stranded := entity.Issue{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Version:     2,
		State:       entity.IssueState{ID: removed.ID, Name: removed.Name, Category: removed.Category},
	}

	h.inState[removed.ID] = []entity.Issue{stranded}
	h.issues.EXPECT().ReassignState(gomock.Any(), removed.ID, replacement.ID).Return(nil)
	h.states.EXPECT().Delete(gomock.Any(), removed.ID).Return(nil)
	h.states.EXPECT().Reposition(gomock.Any(), teamID, gomock.Any()).Return(nil)

	if err := h.service.Remove(
		context.Background(), workspaceID, teamID, removed.ID, replacement.ID,
	); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if len(h.recorded) != 1 {
		t.Fatalf("removing a state left %d entries, want one per issue it held", len(h.recorded))
	}

	entry := h.recorded[0]

	if entry.FromState != removed.Name || entry.ToState != replacement.Name {
		t.Errorf("the entry records %q to %q, want %q to %q", entry.FromState, entry.ToState, removed.Name, replacement.Name)
	}

	if entry.FromCategory != removed.Category || entry.ToCategory != replacement.Category {
		t.Errorf(
			"the entry records the kinds %q to %q, want %q to %q. Deleting a state moves its "+
				"work somewhere that may count differently, and the ledger is what stops a "+
				"later reading of the cycle from pretending it was always there.",
			entry.FromCategory, entry.ToCategory, removed.Category, replacement.Category,
		)
	}
}

package issue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) holding(settings entity.AgentSettings) {
	h.holds = settings
}

func (h *harness) editable(t *testing.T) (uuid.UUID, uuid.UUID, entity.Issue) {
	t.Helper()

	workspaceID := uuid.New()
	issueID := uuid.New()

	issue := entity.Issue{
		ID:          issueID,
		WorkspaceID: workspaceID,
		TeamID:      uuid.New(),
		Version:     1,
		Status:      entity.IssueStatusActive,
		Title:       "Retries drop the idempotency key",
		State: entity.IssueState{
			ID:       uuid.New(),
			Name:     "In progress",
			Category: entity.StateCategoryActive,
		},
	}

	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}
	h.expectGateDecision()

	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(issue, nil).
		AnyTimes()

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(issue, nil).
		AnyTimes()

	return workspaceID, issueID, issue
}

func (h *harness) expectHeld(t *testing.T) *entity.AgentProposal {
	t.Helper()

	captured := &entity.AgentProposal{}

	h.agents.EXPECT().
		GetByAccountID(gomock.Any(), gomock.Any()).
		Return(entity.Agent{ID: uuid.New()}, nil).
		AnyTimes()

	h.proposals.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, held entity.AgentProposal) (entity.AgentProposal, error) {
			held.ID = uuid.New()
			*captured = held

			return held, nil
		}).
		AnyTimes()

	return captured
}

func TestAWriteThatMovesStateIsHeldAsAStateChangeEvenWhenItAlsoRenames(t *testing.T) {
	h := newHarness(t)
	workspaceID, issueID, issue := h.editable(t)

	done := entity.WorkflowState{
		ID:       uuid.New(),
		TeamID:   issue.TeamID,
		Name:     "Done",
		Category: entity.StateCategoryComplete,
	}

	h.states.EXPECT().
		ListByTeamID(gomock.Any(), issue.TeamID).
		Return([]entity.WorkflowState{done}, nil).
		AnyTimes()

	h.holding(entity.AgentSettings{
		HoldStateChanges: entity.AgentHoldAlways,
		HoldIssueEdits:   entity.AgentHoldNever,
	})

	held := h.expectHeld(t)

	renamed := "Retries drop the idempotency key (fixed)"

	_, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &done.ID,
		Title:           &renamed,
	})

	var waiting entity.AgentActionHeldError
	if !errors.As(err, &waiting) {
		t.Fatalf(
			"closing and renaming in one call returned %v; a rename must not carry a close past "+
				"a hold on state changes",
			err,
		)
	}

	if held.Action != entity.AgentActionStateChange {
		t.Fatalf("the proposal was filed as %q, want a state change", held.Action)
	}
}

func TestSettingAFieldToTheValueItAlreadyHasDoesNotChangeHowAWriteIsHeld(t *testing.T) {
	h := newHarness(t)
	workspaceID, issueID, issue := h.editable(t)

	done := entity.WorkflowState{
		ID:       uuid.New(),
		TeamID:   issue.TeamID,
		Name:     "Done",
		Category: entity.StateCategoryComplete,
	}

	h.states.EXPECT().
		ListByTeamID(gomock.Any(), issue.TeamID).
		Return([]entity.WorkflowState{done}, nil).
		AnyTimes()

	h.holding(entity.AgentSettings{
		HoldStateChanges: entity.AgentHoldAlways,
		HoldIssueEdits:   entity.AgentHoldNever,
	})

	held := h.expectHeld(t)

	unchanged := issue.Title

	_, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &done.ID,
		Title:           &unchanged,
	})

	var waiting entity.AgentActionHeldError
	if !errors.As(err, &waiting) {
		t.Fatalf("re-sending the title it already had let a close through: %v", err)
	}

	if held.Action != entity.AgentActionStateChange {
		t.Fatalf(
			"the proposal was filed as %q; re-sending a field unchanged must not reclassify the "+
				"write",
			held.Action,
		)
	}
}

func TestAWriteIsHeldByTheStrongerOfTheTwoPoliciesItTouches(t *testing.T) {
	h := newHarness(t)
	workspaceID, issueID, issue := h.editable(t)

	todo := entity.WorkflowState{
		ID:       uuid.New(),
		TeamID:   issue.TeamID,
		Name:     "Todo",
		Category: entity.StateCategoryNotStarted,
	}

	h.states.EXPECT().
		ListByTeamID(gomock.Any(), issue.TeamID).
		Return([]entity.WorkflowState{todo}, nil).
		AnyTimes()

	h.holding(entity.AgentSettings{
		HoldStateChanges: entity.AgentHoldNever,
		HoldIssueEdits:   entity.AgentHoldAlways,
	})

	h.expectHeld(t)

	renamed := "Something else entirely"

	_, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &todo.ID,
		Title:           &renamed,
	})

	var waiting entity.AgentActionHeldError
	if !errors.As(err, &waiting) {
		t.Fatalf(
			"a rename went through unheld because the same call also moved state, which the team "+
				"does not hold: %v",
			err,
		)
	}
}

func TestAHeldWriteCarriesEveryFieldTheAgentAsked(t *testing.T) {
	h := newHarness(t)
	workspaceID, issueID, issue := h.editable(t)

	h.states.EXPECT().
		ListByTeamID(gomock.Any(), issue.TeamID).
		Return(nil, nil).
		AnyTimes()

	h.holding(entity.AgentSettings{HoldIssueEdits: entity.AgentHoldAlways})

	held := h.expectHeld(t)

	title := "Retries keep the idempotency key"
	description := "The retry path rebuilds the request and loses the header."
	estimate := 3

	_, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		Title:           &title,
		Description:     &description,
		Estimate:        &estimate,
	})

	var waiting entity.AgentActionHeldError
	if !errors.As(err, &waiting) {
		t.Fatalf("the edit was not held: %v", err)
	}

	if held.Change.Description == nil || *held.Change.Description != description {
		t.Fatal(
			"the held change dropped the description, so approving it would apply a different " +
				"edit from the one the agent asked for and the approver was shown",
		)
	}

	if held.Change.Estimate == nil || *held.Change.Estimate != estimate {
		t.Fatal("the held change dropped the estimate")
	}
}

func TestAWriteThatChangesNothingIsNeverHeld(t *testing.T) {
	h := newHarness(t)
	workspaceID, issueID, issue := h.editable(t)

	h.holding(entity.AgentSettings{HoldIssueEdits: entity.AgentHoldAlways})
	h.expectStateWrite(issueID)
	h.captureOverride()

	unchanged := issue.Title

	if _, err := h.service.Update(
		context.Background(), workspaceID, issueID,
		service.UpdateIssueInput{ExpectedVersion: 1, Title: &unchanged},
	); err != nil {
		t.Fatalf("a write that changes nothing was held: %v", err)
	}
}

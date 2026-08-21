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
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	unchanged := issue.Title

	if _, err := h.service.Update(
		context.Background(), workspaceID, issueID,
		service.UpdateIssueInput{ExpectedVersion: 1, Title: &unchanged},
	); err != nil {
		t.Fatalf("a write that changes nothing was held: %v", err)
	}
}

func TestAnAgentsNewIssueIsHeldBeforeItReachesAnybodysBoard(t *testing.T) {
	h := newHarness(t)
	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}
	h.holding(entity.AgentSettings{HoldIssueCreation: entity.AgentHoldAlways})
	h.expectGateDecision()

	workspaceID, teamID := uuid.New(), uuid.New()
	stateID := uuid.New()

	h.states.EXPECT().
		DefaultForTeam(gomock.Any(), teamID).
		Return(entity.WorkflowState{ID: stateID, TeamID: teamID, Name: "Backlog", IsDefault: true}, nil).
		AnyTimes()

	captured := h.expectHeld(t)

	_, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "Retries drop the idempotency key",
		Description: "The second attempt sends a fresh key.",
		Priority:    entity.IssuePriorityHigh,
	})

	var held entity.AgentActionHeldError
	if !errors.As(err, &held) {
		t.Fatalf("Create = %v, want the write held; an unheld create puts the issue on the board", err)
	}

	if captured.Action != entity.AgentActionIssueCreate {
		t.Errorf("held as %q, want issue_create", captured.Action)
	}

	if captured.IssueID != uuid.Nil {
		t.Errorf("the proposal names issue %v, but the issue does not exist yet", captured.IssueID)
	}

	if captured.TeamID != teamID {
		t.Errorf("team = %v, want %v; approving it has to know where the issue lands", captured.TeamID, teamID)
	}

	if captured.Change.Title == nil || *captured.Change.Title != "Retries drop the idempotency key" {
		t.Errorf("the title never reached the proposal: %+v", captured.Change)
	}

	if captured.Change.Description == nil {
		t.Error("the description never reached the proposal, so approving it would file an empty issue")
	}

	if captured.Change.Priority == nil || *captured.Change.Priority != entity.IssuePriorityHigh {
		t.Errorf("the priority never reached the proposal: %+v", captured.Change)
	}
}

func TestATeamThatHoldsEditsDoesNotAlsoHoldNewIssues(t *testing.T) {
	h := newHarness(t)
	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}
	h.holding(entity.AgentSettings{HoldIssueEdits: entity.AgentHoldAlways})
	h.expectGateDecision()

	workspaceID, teamID := uuid.New(), uuid.New()

	h.states.EXPECT().
		DefaultForTeam(gomock.Any(), teamID).
		Return(entity.WorkflowState{ID: uuid.New(), TeamID: teamID, Name: "Backlog", IsDefault: true}, nil).
		AnyTimes()

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)
	h.issues.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, issue entity.Issue) (entity.Issue, error) {
			issue.ID = uuid.New()

			return issue, nil
		})

	if _, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "Retries drop the idempotency key",
	}); err != nil {
		t.Fatalf("Create = %v; holding edits must not quietly hold new issues too", err)
	}
}

func TestAHeldWritePingsThePersonWhoDelegatedTheIssue(t *testing.T) {
	h := newHarness(t)
	workspaceID, issueID, issue := h.editable(t)

	h.states.EXPECT().ListByTeamID(gomock.Any(), issue.TeamID).Return(nil, nil).AnyTimes()
	h.holding(entity.AgentSettings{HoldIssueEdits: entity.AgentHoldAlways})
	h.expectHeld(t)

	delegator := uuid.New()
	h.delegatedBy = delegator

	title := "Retries keep the idempotency key"

	if _, err := h.service.Update(
		context.Background(), workspaceID, issueID,
		service.UpdateIssueInput{ExpectedVersion: 1, Title: &title},
	); err == nil {
		t.Fatal("the edit was not held")
	}

	var waiting []entity.NotificationEvent

	for _, event := range h.notified {
		if event.Kind == entity.NotificationKindApprovalWaiting {
			waiting = append(waiting, event)
		}
	}

	if len(waiting) != 1 {
		t.Fatalf("a held write raised %d approval notices, want exactly one", len(waiting))
	}

	if waiting[0].Target != delegator {
		t.Fatal("the notice does not reach the person who delegated the work")
	}

	if waiting[0].Subject.ID != issueID {
		t.Fatal("the notice does not point at the issue that is waiting")
	}
}

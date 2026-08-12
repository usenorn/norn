package issue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) blockedBy(statement string) uuid.UUID {
	checkID := uuid.New()

	h.blocking = []entity.Check{{
		ID:        checkID,
		Statement: statement,
		Method:    entity.CheckMethodCommand,
		Approval:  entity.CheckApprovalApproved,
	}}

	return checkID
}

func (h *harness) completing(t *testing.T) (uuid.UUID, uuid.UUID, entity.WorkflowState) {
	t.Helper()

	workspaceID := uuid.New()
	issueID := uuid.New()

	done := entity.WorkflowState{
		ID:       uuid.New(),
		TeamID:   uuid.New(),
		Name:     "Done",
		Category: entity.StateCategoryComplete,
	}

	issue := entity.Issue{
		ID:          issueID,
		WorkspaceID: workspaceID,
		TeamID:      done.TeamID,
		Version:     1,
		Status:      entity.IssueStatusActive,
		State: entity.IssueState{
			ID:       uuid.New(),
			Name:     "In progress",
			Category: entity.StateCategoryActive,
		},
	}

	h.expectGateDecision()

	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(issue, nil).
		AnyTimes()

	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(issue, nil).
		AnyTimes()

	h.states.EXPECT().
		ListByTeamID(gomock.Any(), issue.TeamID).
		Return([]entity.WorkflowState{done}, nil).
		AnyTimes()

	h.issues.EXPECT().
		ListChildren(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	return workspaceID, issueID, done
}

func TestAnAgentCannotMoveAnIssueIntoCompletionWhileAnApprovedCheckIsUnproven(t *testing.T) {
	h := newHarness(t)
	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}

	workspaceID, issueID, done := h.completing(t)
	h.blockedBy("payments retry without duplicating a charge")

	_, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &done.ID,
	})

	var unproven entity.IssueChecksUnprovenError
	if !errors.As(err, &unproven) {
		t.Fatalf("an agent closing an unproven issue returned %v, want it refused", err)
	}

	if len(unproven.Checks) != 1 ||
		unproven.Checks[0].Statement != "payments retry without duplicating a charge" {
		t.Fatalf("the refusal did not name the check in the way: %+v", unproven.Checks)
	}
}

func TestAPersonClosingAnIssueWithUnprovenChecksSucceedsFirstTime(t *testing.T) {
	h := newHarness(t)

	workspaceID, issueID, done := h.completing(t)
	h.blockedBy("payments retry without duplicating a charge")
	h.expectStateWrite(issueID)

	overrides := h.captureOverride()

	if _, err := h.service.Update(
		context.Background(), workspaceID, issueID,
		service.UpdateIssueInput{ExpectedVersion: 1, StateID: &done.ID},
	); err != nil {
		t.Fatalf("a person was refused: %v", err)
	}

	if len(*overrides) != 1 {
		t.Fatal("closing with unproven checks left nothing on the timeline")
	}

	entry := (*overrides)[0]

	if entry.ToValue != "payments retry without duplicating a charge" {
		t.Fatalf("the override entry does not name the checks: %q", entry.ToValue)
	}

	if entry.FromValue == entity.OverrideAcknowledged {
		t.Fatal("an unacknowledged override was recorded as though the person had been shown them")
	}
}

func TestAnOverrideRecordsWhetherThePersonWasShownTheChecks(t *testing.T) {
	h := newHarness(t)

	workspaceID, issueID, done := h.completing(t)
	h.blockedBy("payments retry without duplicating a charge")
	h.expectStateWrite(issueID)

	overrides := h.captureOverride()

	if _, err := h.service.Update(
		context.Background(), workspaceID, issueID,
		service.UpdateIssueInput{
			ExpectedVersion:           1,
			StateID:                   &done.ID,
			AcknowledgeUnprovenChecks: true,
		},
	); err != nil {
		t.Fatalf("update: %v", err)
	}

	if len(*overrides) != 1 || (*overrides)[0].FromValue != entity.OverrideAcknowledged {
		t.Fatal("a confirmed override is not distinguishable from a blind one")
	}
}

func TestApprovingAQueuedCompletionAppliesItEvenThoughItsChecksAreUnproven(t *testing.T) {
	h := newHarness(t)
	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}

	workspaceID, issueID, done := h.completing(t)
	h.blockedBy("payments retry without duplicating a charge")
	h.expectStateWrite(issueID)

	overrides := h.captureOverride()

	approver := uuid.New()
	acting := identity.WithApproval(context.Background(), approver)

	if _, err := h.service.Update(acting, workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &done.ID,
	}); err != nil {
		t.Fatalf("an approved completion was refused: %v", err)
	}

	if len(*overrides) != 1 {
		t.Fatal("an approved completion recorded no override")
	}

	entry := (*overrides)[0]

	if entry.Actor.AccountID != approver || entry.Actor.Kind != entity.ActorKindUser {
		t.Fatalf(
			"the override names %s (%s), want the person who approved it",
			entry.Actor.AccountID, entry.Actor.Kind,
		)
	}
}

func TestAMoveThatIsNotIntoCompletionIgnoresChecksEntirely(t *testing.T) {
	h := newHarness(t)
	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}

	workspaceID := uuid.New()
	issueID := uuid.New()

	started := entity.WorkflowState{
		ID:       uuid.New(),
		TeamID:   uuid.New(),
		Name:     "In progress",
		Category: entity.StateCategoryActive,
	}

	issue := entity.Issue{
		ID:          issueID,
		WorkspaceID: workspaceID,
		TeamID:      started.TeamID,
		Version:     1,
		Status:      entity.IssueStatusActive,
		State: entity.IssueState{
			ID:       uuid.New(),
			Name:     "Todo",
			Category: entity.StateCategoryNotStarted,
		},
	}

	h.expectGateDecision()
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil).AnyTimes()
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil).AnyTimes()
	h.states.EXPECT().ListByTeamID(gomock.Any(), issue.TeamID).Return([]entity.WorkflowState{started}, nil).AnyTimes()
	h.expectStateWrite(issueID)
	h.captureOverride()

	if _, err := h.service.Update(
		context.Background(), workspaceID, issueID,
		service.UpdateIssueInput{ExpectedVersion: 1, StateID: &started.ID},
	); err != nil {
		t.Fatalf("moving an issue to an unfinished state was refused: %v", err)
	}
}

func (h *harness) expectGateDecision() {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor: h.actor,
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()
}

func (h *harness) expectStateWrite(issueID uuid.UUID) {
	h.issues.EXPECT().
		Update(gomock.Any(), issueID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()
}

func (h *harness) captureOverride() *[]entity.Activity {
	recorded := &[]entity.Activity{}

	h.activity.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.Activity) error {
			if entry.Kind == entity.ActivityKindChecksOverridden {
				*recorded = append(*recorded, entry)
			}

			return nil
		}).
		AnyTimes()

	return recorded
}

func (h *harness) awaitingApprovalOf(statement string) uuid.UUID {
	checkID := uuid.New()

	h.blocking = []entity.Check{{
		ID:         checkID,
		Statement:  statement,
		Method:     entity.CheckMethodCommand,
		Approval:   entity.CheckApprovalPending,
		Resolution: entity.CheckResolutionNone,
	}}

	return checkID
}

func TestAnAgentCannotFinishAnIssueWhoseCriteriaNobodyHasApproved(t *testing.T) {
	h := newHarness(t)
	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}

	workspaceID, issueID, done := h.completing(t)
	h.awaitingApprovalOf("the importer refuses a duplicate row")

	_, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &done.ID,
	})

	var unratified entity.IssueChecksUnratifiedError
	if !errors.As(err, &unratified) {
		t.Fatalf("an agent closing against an unapproved contract returned %v, want it refused", err)
	}

	if len(unratified.Checks) != 1 ||
		unratified.Checks[0].Statement != "the importer refuses a duplicate row" {
		t.Fatalf("the refusal did not name what is waiting: %+v", unratified.Checks)
	}
}

func TestADeclinedCriterionNeverWedgesTheIssue(t *testing.T) {
	h := newHarness(t)
	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}

	workspaceID, issueID, done := h.completing(t)
	h.blocking = []entity.Check{{
		ID:         uuid.New(),
		Statement:  "the importer refuses a duplicate row",
		Method:     entity.CheckMethodCommand,
		Approval:   entity.CheckApprovalDeclined,
		Resolution: entity.CheckResolutionNone,
	}}
	h.expectStateWrite(issueID)
	h.captureOverride()

	if _, err := h.service.Update(
		context.Background(), workspaceID, issueID,
		service.UpdateIssueInput{ExpectedVersion: 1, StateID: &done.ID},
	); err != nil {
		t.Fatalf("a declined criterion refused the close: %v", err)
	}
}

func TestAPersonIsNeverRefusedByAnUnapprovedContract(t *testing.T) {
	h := newHarness(t)

	workspaceID, issueID, done := h.completing(t)
	h.awaitingApprovalOf("the importer refuses a duplicate row")
	h.expectStateWrite(issueID)
	h.captureOverride()

	if _, err := h.service.Update(
		context.Background(), workspaceID, issueID,
		service.UpdateIssueInput{ExpectedVersion: 1, StateID: &done.ID},
	); err != nil {
		t.Fatalf("a person was refused: %v", err)
	}
}

func TestApprovingAQueuedCompletionRatifiesTheCriteriaItWaitedOn(t *testing.T) {
	h := newHarness(t)
	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}

	workspaceID, issueID, done := h.completing(t)
	h.awaitingApprovalOf("the importer refuses a duplicate row")
	h.expectStateWrite(issueID)
	h.captureOverride()

	acting := identity.WithApproval(context.Background(), uuid.New())

	if _, err := h.service.Update(acting, workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &done.ID,
	}); err != nil {
		t.Fatalf("an approved completion was refused: %v", err)
	}
}

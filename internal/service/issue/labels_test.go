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

func labelled(workspaceID uuid.UUID, name string) entity.Label {
	return entity.Label{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Name:        name,
		Color:       entity.LabelColorCyan,
	}
}

func TestAnIssueMayCarrySeveralLabelsFromDifferentGroups(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	issueID := uuid.New()
	severity := uuid.New()
	area := uuid.New()

	blocker := labelled(workspaceID, "Blocker")
	blocker.GroupID = severity
	backend := labelled(workspaceID, "Backend")
	backend.GroupID = area
	loose := labelled(workspaceID, "Needs spec")

	wanted := []entity.Label{blocker, backend, loose}
	ids := []uuid.UUID{blocker.ID, backend.ID, loose.ID}

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.expectLabelLock(workspaceID, issueID, teamID)
	h.labels.EXPECT().ListByIDs(gomock.Any(), workspaceID, ids).Return(wanted, nil)

	var stored []entity.Label

	h.labels.EXPECT().
		SetForIssue(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.Issue, labels []entity.Label) error {
			stored = labels

			return nil
		})

	applied, err := h.service.SetLabels(context.Background(), workspaceID, issueID, service.SetIssueLabelsInput{ExpectedVersion: 1, LabelIDs: ids})
	if err != nil {
		t.Fatalf("SetLabels: %v", err)
	}

	if len(stored) != 3 || len(applied) != 3 {
		t.Fatalf("stored %d and returned %d labels, want 3 of each", len(stored), len(applied))
	}
}

func TestASubmissionCarryingTwoLabelsFromOneGroupIsRefusedAndNothingIsWritten(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	issueID := uuid.New()
	severity := uuid.New()

	blocker := labelled(workspaceID, "Blocker")
	blocker.GroupID = severity
	major := labelled(workspaceID, "Major")
	major.GroupID = severity

	ids := []uuid.UUID{blocker.ID, major.ID}

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.expectLabelLock(workspaceID, issueID, teamID)
	h.labels.EXPECT().
		ListByIDs(gomock.Any(), workspaceID, ids).
		Return([]entity.Label{blocker, major}, nil)

	_, err := h.service.SetLabels(context.Background(), workspaceID, issueID, service.SetIssueLabelsInput{ExpectedVersion: 1, LabelIDs: ids})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("SetLabels error = %v, want a ValidationError", err)
	}

	if validation.Fields[0].Field != "labelIds" {
		t.Fatalf("validation names %q, want labelIds", validation.Fields[0].Field)
	}
}

func TestATeamScopedLabelCannotBeAppliedToAnotherTeamsIssue(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	mobile := uuid.New()
	platform := uuid.New()
	issueID := uuid.New()

	crash := labelled(workspaceID, "Crash")
	crash.TeamID = mobile

	ids := []uuid.UUID{crash.ID}

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.expectLabelLock(workspaceID, issueID, platform)
	h.labels.EXPECT().ListByIDs(gomock.Any(), workspaceID, ids).Return([]entity.Label{crash}, nil)

	_, err := h.service.SetLabels(context.Background(), workspaceID, issueID, service.SetIssueLabelsInput{ExpectedVersion: 1, LabelIDs: ids})

	if !errors.Is(err, entity.ErrLabelOutOfScope) {
		t.Fatalf("SetLabels error = %v, want ErrLabelOutOfScope", err)
	}
}

func TestAWorkspaceLabelAppliesToAnIssueInAnyTeam(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	issueID := uuid.New()
	bug := labelled(workspaceID, "Bug")
	ids := []uuid.UUID{bug.ID}

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.expectLabelLock(workspaceID, issueID, uuid.New())
	h.labels.EXPECT().ListByIDs(gomock.Any(), workspaceID, ids).Return([]entity.Label{bug}, nil)
	h.labels.EXPECT().SetForIssue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.service.SetLabels(context.Background(), workspaceID, issueID, service.SetIssueLabelsInput{ExpectedVersion: 1, LabelIDs: ids}); err != nil {
		t.Fatalf("SetLabels: %v", err)
	}
}

func TestAnUnknownLabelIsRefusedRatherThanSilentlyDropped(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	issueID := uuid.New()
	known := labelled(workspaceID, "Bug")
	ids := []uuid.UUID{known.ID, uuid.New()}

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.expectLabelLock(workspaceID, issueID, uuid.New())
	h.labels.EXPECT().ListByIDs(gomock.Any(), workspaceID, ids).Return([]entity.Label{known}, nil)

	_, err := h.service.SetLabels(context.Background(), workspaceID, issueID, service.SetIssueLabelsInput{ExpectedVersion: 1, LabelIDs: ids})

	if !errors.Is(err, entity.ErrLabelNotFound) {
		t.Fatalf("SetLabels error = %v, want ErrLabelNotFound", err)
	}
}

func TestClearingEveryLabelIsExpressibleAndWritesAnEmptySet(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	issueID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.expectLabelLock(workspaceID, issueID, uuid.New(), labelled(workspaceID, "Needs spec"))
	h.labels.EXPECT().
		ListByIDs(gomock.Any(), workspaceID, []uuid.UUID{}).
		Return([]entity.Label{}, nil)

	cleared := false

	h.labels.EXPECT().
		SetForIssue(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.Issue, labels []entity.Label) error {
			cleared = len(labels) == 0

			return nil
		})

	if _, err := h.service.SetLabels(context.Background(), workspaceID, issueID, service.SetIssueLabelsInput{ExpectedVersion: 1, LabelIDs: []uuid.UUID{}}); err != nil {
		t.Fatalf("SetLabels: %v", err)
	}

	if !cleared {
		t.Fatal("submitting an empty set must clear the issue's labels, not leave them alone")
	}
}

func TestApplyingLabelsRequiresTheSameRightAsMovingAnIssue(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()

	var request entity.AccessRequest

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req entity.AccessRequest) (entity.Decision, error) {
			request = req

			return entity.Decision{}, entity.ErrAccountForbidden
		})

	if _, err := h.service.SetLabels(context.Background(), workspaceID, uuid.New(), service.SetIssueLabelsInput{}); err == nil {
		t.Fatal("SetLabels succeeded without a decision")
	}

	if request.Resource != entity.ResourceIssue || request.Action != entity.ActionManage {
		t.Fatalf(
			"SetLabels asked for %s:%s, want issue:manage — labelling an issue is changing it",
			request.Resource, request.Action,
		)
	}

	if !request.Scoped {
		t.Fatal("SetLabels must ask for a team-scoped decision so a private team's issues stay hidden")
	}
}

func (h *harness) expectLabelLock(
	workspaceID, issueID, teamID uuid.UUID,
	existing ...entity.Label,
) {
	issue := entity.Issue{
		ID:          issueID,
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Version:     1,
		Labels:      existing,
	}

	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil)
	h.issues.EXPECT().StampLabels(gomock.Any(), issueID, 1, gomock.Any()).Return(nil).AnyTimes()
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
}

func TestChangingTheLabelsIsWrittenIntoTheHistoryAndBumpsTheRow(t *testing.T) {
	h := newHarness(t)

	workspaceID, issueID, teamID := uuid.New(), uuid.New(), uuid.New()

	before := labelled(workspaceID, "Needs spec")
	after := labelled(workspaceID, "Blocker")

	issue := entity.Issue{
		ID:          issueID,
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Version:     4,
		Labels:      []entity.Label{before},
	}

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil)
	h.labels.EXPECT().
		ListByIDs(gomock.Any(), workspaceID, []uuid.UUID{after.ID}).
		Return([]entity.Label{after}, nil)
	h.labels.EXPECT().SetForIssue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	stamped := 0
	h.issues.EXPECT().
		StampLabels(gomock.Any(), issueID, 4, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, _ int, _ time.Time) error {
			stamped++

			return nil
		})

	var recorded entity.IssueActivity

	h.activity.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.IssueActivity) error {
			recorded = entry

			return nil
		})

	if _, err := h.service.SetLabels(context.Background(), workspaceID, issueID, service.SetIssueLabelsInput{
		ExpectedVersion: 4,
		LabelIDs:        []uuid.UUID{after.ID},
	}); err != nil {
		t.Fatalf("SetLabels: %v", err)
	}

	if stamped != 1 {
		t.Fatal(
			"labelling did not bump the issue row. Nothing stamps field_versions.labels, so a " +
				"concurrent team move reading the old version is not told the labels moved under it " +
				"and drops them silently.",
		)
	}

	if recorded.Field != entity.IssueFieldLabels {
		t.Fatalf("recorded %q, want the labels field", recorded.Field)
	}

	if recorded.FromValue != "Needs spec" || recorded.ToValue != "Blocker" {
		t.Fatalf(
			"history says %q → %q, want \"Needs spec\" → \"Blocker\"; a reader cannot tell what "+
				"the labels were before",
			recorded.FromValue, recorded.ToValue,
		)
	}

	if recorded.Version != 5 {
		t.Fatalf("recorded at version %d, want 5 — the version the change produced", recorded.Version)
	}
}

func TestRelabellingAgainstAStaleReadIsRefused(t *testing.T) {
	h := newHarness(t)

	workspaceID, issueID := uuid.New(), uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:            issueID,
			WorkspaceID:   workspaceID,
			Version:       7,
			FieldVersions: map[string]int{entity.IssueFieldLabels: 7},
		}, nil)

	_, err := h.service.SetLabels(context.Background(), workspaceID, issueID, service.SetIssueLabelsInput{
		ExpectedVersion: 6,
		LabelIDs:        []uuid.UUID{uuid.New()},
	})

	var stale entity.IssueStaleError
	if !errors.As(err, &stale) {
		t.Fatalf("SetLabels error = %v, want a stale-read refusal", err)
	}

	if len(stale.Conflicts) == 0 || stale.Conflicts[0] != entity.IssueFieldLabels {
		t.Fatalf("conflicts = %v, want the labels field named", stale.Conflicts)
	}
}

func TestReapplyingTheSameLabelsChangesNothingAndWritesNoHistory(t *testing.T) {
	h := newHarness(t)

	workspaceID, issueID := uuid.New(), uuid.New()
	existing := labelled(workspaceID, "Blocker")

	issue := entity.Issue{
		ID:          issueID,
		WorkspaceID: workspaceID,
		Version:     2,
		Labels:      []entity.Label{existing},
	}

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil)
	h.labels.EXPECT().
		ListByIDs(gomock.Any(), workspaceID, []uuid.UUID{existing.ID}).
		Return([]entity.Label{existing}, nil)

	if _, err := h.service.SetLabels(context.Background(), workspaceID, issueID, service.SetIssueLabelsInput{
		ExpectedVersion: 2,
		LabelIDs:        []uuid.UUID{existing.ID},
	}); err != nil {
		t.Fatalf("SetLabels: %v", err)
	}
}

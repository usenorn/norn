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

func TestFilingAnIssueUnderItsOwnDescendantIsRefusedAtAnyDepth(t *testing.T) {
	for _, depth := range []int{1, 2, 4} {
		h := newHarness(t)

		workspaceID, issueID, parentID := uuid.New(), uuid.New(), uuid.New()

		ancestry := make([]uuid.UUID, 0, depth)
		for range depth - 1 {
			ancestry = append(ancestry, uuid.New())
		}

		ancestry = append(ancestry, issueID)

		h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
		h.issues.EXPECT().
			LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
			Return(entity.Issue{ID: issueID, Version: 1, Depth: 1}, nil)
		h.issues.EXPECT().
			GetVisible(gomock.Any(), workspaceID, parentID, gomock.Any()).
			Return(entity.Issue{ID: parentID, Status: entity.IssueStatusActive, Depth: 2}, nil)
		h.issues.EXPECT().Ancestors(gomock.Any(), parentID).Return(ancestry, nil)

		_, err := h.service.SetParent(context.Background(), workspaceID, issueID, service.SetIssueParentInput{
			ExpectedVersion: 1,
			ParentID:        &parentID,
		})

		if !errors.Is(err, entity.ErrIssueParentCycle) {
			t.Errorf(
				"an issue %d level(s) above the chosen parent was accepted (%v). A cycle at any "+
					"depth makes the tree unwalkable, not just a direct one.",
				depth, err,
			)
		}
	}
}

func TestAnIssueCannotBecomeItsOwnParent(t *testing.T) {
	h := newHarness(t)

	workspaceID, issueID := uuid.New(), uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	_, err := h.service.SetParent(context.Background(), workspaceID, issueID, service.SetIssueParentInput{
		ExpectedVersion: 1,
		ParentID:        &issueID,
	})

	if !errors.Is(err, entity.ErrIssueParentCycle) {
		t.Fatalf("SetParent error = %v, want a cycle refusal", err)
	}
}

func TestMovingASubTreeIsRefusedWhenItsDeepestChildWouldBreachTheCap(t *testing.T) {
	h := newHarness(t)

	workspaceID, issueID, parentID := uuid.New(), uuid.New(), uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{ID: issueID, Version: 1, Depth: 1}, nil)
	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, parentID, gomock.Any()).
		Return(entity.Issue{ID: parentID, Status: entity.IssueStatusActive, Depth: entity.IssueMaxDepth - 2}, nil)
	h.issues.EXPECT().Ancestors(gomock.Any(), parentID).Return(nil, nil)
	h.issues.EXPECT().SubtreeHeight(gomock.Any(), issueID).Return(2, nil)

	_, err := h.service.SetParent(context.Background(), workspaceID, issueID, service.SetIssueParentInput{
		ExpectedVersion: 1,
		ParentID:        &parentID,
	})

	var tooDeep entity.IssueTooDeepError
	if !errors.As(err, &tooDeep) {
		t.Fatalf(
			"SetParent error = %v, want a depth refusal. The issue itself would land within the "+
				"cap; only its descendants breach it, which is exactly the case a naive check misses.",
			err,
		)
	}

	if tooDeep.Depth <= entity.IssueMaxDepth {
		t.Fatalf("refusal reports depth %d, which is within the cap of %d", tooDeep.Depth, tooDeep.Max)
	}
}

func TestAnIssueCannotBeFiledUnderAnArchivedOrDeletedParent(t *testing.T) {
	for _, status := range []entity.IssueStatus{entity.IssueStatusArchived, entity.IssueStatusPendingDeletion} {
		h := newHarness(t)

		workspaceID, issueID, parentID := uuid.New(), uuid.New(), uuid.New()

		h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
		h.issues.EXPECT().
			LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
			Return(entity.Issue{ID: issueID, Version: 1, Depth: 1}, nil)
		h.issues.EXPECT().
			GetVisible(gomock.Any(), workspaceID, parentID, gomock.Any()).
			Return(entity.Issue{ID: parentID, Status: status, Depth: 1}, nil)

		_, err := h.service.SetParent(context.Background(), workspaceID, issueID, service.SetIssueParentInput{
			ExpectedVersion: 1,
			ParentID:        &parentID,
		})

		if !errors.Is(err, entity.ErrIssueParentNotActive) {
			t.Errorf("filing under a %s parent gave %v, want a refusal", status, err)
		}
	}
}

func TestReparentingWritesHistoryOnTheChildTheOldParentAndTheNewParent(t *testing.T) {
	h := newHarness(t)

	workspaceID, issueID := uuid.New(), uuid.New()
	oldParentID, newParentID := uuid.New(), uuid.New()

	child := entity.Issue{
		ID:              issueID,
		WorkspaceID:     workspaceID,
		Version:         3,
		Depth:           2,
		ReferenceKey:    "MOB",
		Number:          7,
		ParentIssueID:   oldParentID,
		ParentReference: "MOB-1",
	}

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(child, nil)
	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, newParentID, gomock.Any()).
		Return(entity.Issue{
			ID: newParentID, Status: entity.IssueStatusActive, Depth: 2, ReferenceKey: "PLT", Number: 4,
		}, nil)
	h.issues.EXPECT().Ancestors(gomock.Any(), newParentID).Return(nil, nil)
	h.issues.EXPECT().SubtreeHeight(gomock.Any(), issueID).Return(1, nil)

	var wroteDepth int

	h.issues.EXPECT().
		SetParent(gomock.Any(), issueID, 3, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, _ int, parent *uuid.UUID, depth int, _ any,
		) error {
			wroteDepth = depth

			return nil
		})

	recorded := map[uuid.UUID]entity.IssueActivityKind{}

	h.activity.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.IssueActivity) error {
			recorded[entry.IssueID] = entry.Kind

			return nil
		}).
		Times(3)

	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(child, nil)

	if _, err := h.service.SetParent(context.Background(), workspaceID, issueID, service.SetIssueParentInput{
		ExpectedVersion: 3,
		ParentID:        &newParentID,
	}); err != nil {
		t.Fatalf("SetParent: %v", err)
	}

	if wroteDepth != 3 {
		t.Fatalf("child written at depth %d, want one below its new parent at depth 2", wroteDepth)
	}

	if recorded[issueID] != entity.IssueActivityKindPropertyChanged {
		t.Error("the child's own history does not record that its parent changed")
	}

	if recorded[oldParentID] != entity.IssueActivityKindChildRemoved {
		t.Error("the former parent's history does not record losing the child")
	}

	if recorded[newParentID] != entity.IssueActivityKindChildAdded {
		t.Error("the new parent's history does not record gaining the child")
	}
}

func TestCompletingAParentIsRefusedWhileChildrenAreOpenAndAcceptedOnAcknowledgement(t *testing.T) {
	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()
	doneID := uuid.New()

	done := entity.WorkflowState{
		ID: doneID, TeamID: teamID, Name: "Done", Category: entity.StateCategoryComplete,
	}
	issue := entity.Issue{
		ID:      issueID,
		TeamID:  teamID,
		Version: 1,
		State:   entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryActive},
	}
	openChild := entity.Issue{
		ReferenceKey: "MOB", Number: 9,
		State: entity.IssueState{Category: entity.StateCategoryActive},
	}

	refused := newHarness(t)
	refused.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	refused.issues.EXPECT().LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil)
	refused.states.EXPECT().ListByTeamID(gomock.Any(), teamID).Return([]entity.WorkflowState{done}, nil)
	refused.issues.EXPECT().
		ListChildren(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return([]entity.Issue{openChild}, nil)

	_, err := refused.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &doneID,
	})

	var blocked entity.IssueChildrenOpenError
	if !errors.As(err, &blocked) {
		t.Fatalf("Update error = %v, want the open children to block completion", err)
	}

	if len(blocked.Children) != 1 || blocked.Children[0].Reference() != "MOB-9" {
		t.Fatalf("refusal named %v, want MOB-9", blocked.Children)
	}

	accepted := newHarness(t)
	accepted.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	accepted.issues.EXPECT().LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(issue, nil)
	accepted.states.EXPECT().ListByTeamID(gomock.Any(), teamID).Return([]entity.WorkflowState{done}, nil)
	accepted.issues.EXPECT().
		Update(gomock.Any(), issueID, 1, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	accepted.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	accepted.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{}, nil)

	if _, err := accepted.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion:         1,
		StateID:                 &doneID,
		AcknowledgeOpenChildren: true,
	}); err != nil {
		t.Fatalf("acknowledged completion: %v", err)
	}
}

func TestAnAlreadyCompleteParentIsNotReCheckedForOpenChildren(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID, issueID := uuid.New(), uuid.New(), uuid.New()
	doneID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:      issueID,
			TeamID:  teamID,
			Version: 1,
			State:   entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryComplete},
		}, nil)
	h.states.EXPECT().
		ListByTeamID(gomock.Any(), teamID).
		Return([]entity.WorkflowState{
			{ID: doneID, TeamID: teamID, Name: "Shipped", Category: entity.StateCategoryComplete},
		}, nil)
	h.issues.EXPECT().Update(gomock.Any(), issueID, 1, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).Return(entity.Issue{}, nil)

	if _, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		StateID:         &doneID,
	}); err != nil {
		t.Fatalf(
			"moving between two complete states was blocked (%v). The parent is already finished; "+
				"shuffling it within the complete category cannot newly strand its children.",
			err,
		)
	}
}

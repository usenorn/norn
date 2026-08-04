package issuerelation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestRecordingThatOneIssueBlocksAnotherWritesASingleRowNotTwo(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	a, b := issue("MOB", 1), issue("MOB", 2)

	h.expectScope(workspaceID)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, a.ID, gomock.Any()).Return(a, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, b.ID, gomock.Any()).Return(b, nil)

	var written []entity.StoredIssueRelation

	h.expectNoRelationHeld()
	h.relations.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r entity.StoredIssueRelation) (entity.StoredIssueRelation, error) {
			written = append(written, r)
			r.ID = uuid.New()

			return r, nil
		})

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	created, err := h.service.Add(context.Background(), workspaceID, a.ID, service.AddIssueRelationInput{
		Kind:          entity.IssueRelationViewBlocks,
		CounterpartID: b.ID,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if len(written) != 1 {
		t.Fatalf(
			"%d rows written. The inverse is computed on read; writing a second row means the "+
				"two can drift apart and removing one leaves the other behind.",
			len(written),
		)
	}

	if written[0].SourceIssueID != a.ID || written[0].TargetIssueID != b.ID {
		t.Fatal("the blocker must be the source, or the direction reads backwards")
	}

	if created.Kind != entity.IssueRelationViewBlocks {
		t.Fatalf("returned kind %q, want blocks", created.Kind)
	}
}

func TestSayingAnIssueIsBlockedByAnotherStoresTheBlockerAsTheSource(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	mine, blocker := issue("MOB", 5), issue("PLT", 9)

	h.expectScope(workspaceID)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, mine.ID, gomock.Any()).Return(mine, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, blocker.ID, gomock.Any()).Return(blocker, nil)

	var written entity.StoredIssueRelation

	h.expectNoRelationHeld()
	h.relations.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r entity.StoredIssueRelation) (entity.StoredIssueRelation, error) {
			written = r

			return r, nil
		})
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	created, err := h.service.Add(context.Background(), workspaceID, mine.ID, service.AddIssueRelationInput{
		Kind:          entity.IssueRelationViewBlockedBy,
		CounterpartID: blocker.ID,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if written.Kind != entity.IssueRelationBlocks {
		t.Fatalf("stored kind %q; only the three canonical kinds may reach the database", written.Kind)
	}

	if written.SourceIssueID != blocker.ID || written.TargetIssueID != mine.ID {
		t.Fatal(
			"asking to be blocked by an issue stored the direction unchanged, so the blocker " +
				"and the blocked are the wrong way round",
		)
	}

	if created.Kind != entity.IssueRelationViewBlockedBy {
		t.Fatalf("read back as %q, want blocked_by", created.Kind)
	}
}

func TestASymmetricRelationLandsOnTheSameRowFromEitherSide(t *testing.T) {
	workspaceID := uuid.New()
	a, b := issue("MOB", 1), issue("MOB", 2)

	stored := func(subject, counterpart entity.Issue) entity.StoredIssueRelation {
		h := newHarness(t)

		h.expectScope(workspaceID)
		h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, subject.ID, gomock.Any()).Return(subject, nil)
		h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, counterpart.ID, gomock.Any()).Return(counterpart, nil)

		var written entity.StoredIssueRelation

		h.expectNoRelationHeld()
		h.relations.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, r entity.StoredIssueRelation) (entity.StoredIssueRelation, error) {
				written = r

				return r, nil
			})
		h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).Times(2)

		if _, err := h.service.Add(context.Background(), workspaceID, subject.ID, service.AddIssueRelationInput{
			Kind:          entity.IssueRelationViewRelatesTo,
			CounterpartID: counterpart.ID,
		}); err != nil {
			t.Fatalf("Add: %v", err)
		}

		return written
	}

	forward, backward := stored(a, b), stored(b, a)

	if forward.SourceIssueID != backward.SourceIssueID ||
		forward.TargetIssueID != backward.TargetIssueID {
		t.Fatalf(
			"relating A to B stored (%s -> %s) but relating B to A stored (%s -> %s). A symmetric "+
				"relation must normalise to one row, or the pair index will not see them as the same.",
			forward.SourceIssueID, forward.TargetIssueID,
			backward.SourceIssueID, backward.TargetIssueID,
		)
	}
}

func TestARelationToAnIssueTheActorCannotSeeIsReportedAsMissingNotForbidden(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	mine := issue("MOB", 1)
	hidden := uuid.New()

	h.expectScope(workspaceID)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, mine.ID, gomock.Any()).Return(mine, nil)
	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, hidden, gomock.Any()).
		Return(entity.Issue{}, entity.ErrIssueNotFound)

	_, err := h.service.Add(context.Background(), workspaceID, mine.ID, service.AddIssueRelationInput{
		Kind:          entity.IssueRelationViewBlocks,
		CounterpartID: hidden,
	})

	if !errors.Is(err, entity.ErrIssueNotFound) {
		t.Fatalf("Add error = %v, want ErrIssueNotFound", err)
	}

	if errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatal(
			"an issue the actor cannot see was refused as forbidden rather than missing, which " +
				"confirms it exists and lets anyone probe for issues on private teams",
		)
	}
}

func TestASecondRelationOnThePairIsRefusedAndNamesTheOneAlreadyHeld(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	a, b := issue("MOB", 1), issue("MOB", 2)

	h.expectScope(workspaceID)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, a.ID, gomock.Any()).Return(a, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, b.ID, gomock.Any()).Return(b, nil)
	h.relations.EXPECT().
		FindPair(gomock.Any(), workspaceID, a.ID, b.ID, gomock.Any()).
		Return(entity.IssueRelation{Kind: entity.IssueRelationViewBlocks, Issue: b}, nil)

	_, err := h.service.Add(context.Background(), workspaceID, a.ID, service.AddIssueRelationInput{
		Kind:          entity.IssueRelationViewRelatesTo,
		CounterpartID: b.ID,
	})

	var held entity.IssueRelationExistsError
	if !errors.As(err, &held) {
		t.Fatalf("Add error = %v, want the refusal to carry what is already held", err)
	}

	if held.Reference != "MOB-2" || held.Kind != entity.IssueRelationViewBlocks {
		t.Fatalf(
			"refusal says %q %q; without naming the existing relation the caller cannot tell "+
				"what to remove first",
			held.Kind, held.Reference,
		)
	}
}

func TestAnIssueCannotBeRelatedToItself(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	mine := issue("MOB", 1)

	h.expectScope(workspaceID)

	_, err := h.service.Add(context.Background(), workspaceID, mine.ID, service.AddIssueRelationInput{
		Kind:          entity.IssueRelationViewBlocks,
		CounterpartID: mine.ID,
	})

	if !errors.Is(err, entity.ErrIssueRelationSelf) {
		t.Fatalf("Add error = %v, want a self-relation refusal", err)
	}
}

func TestRemovingFromEitherEndDeletesTheOneRowAndTellsBothIssues(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	a, b := issue("MOB", 1), issue("MOB", 2)
	relationID := uuid.New()

	h.expectScope(workspaceID)
	h.relations.EXPECT().
		GetByID(gomock.Any(), workspaceID, relationID).
		Return(entity.StoredIssueRelation{
			ID:            relationID,
			WorkspaceID:   workspaceID,
			SourceIssueID: a.ID,
			TargetIssueID: b.ID,
			Kind:          entity.IssueRelationBlocks,
		}, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, b.ID, gomock.Any()).Return(b, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, a.ID, gomock.Any()).Return(a, nil)

	deleted := 0
	h.relations.EXPECT().
		Delete(gomock.Any(), relationID).
		DoAndReturn(func(_ context.Context, _ uuid.UUID) error {
			deleted++

			return nil
		})

	told := map[uuid.UUID]entity.IssueActivityKind{}

	h.activity.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.IssueActivity) error {
			told[entry.IssueID] = entry.Kind

			return nil
		}).
		Times(2)

	if err := h.service.Remove(context.Background(), workspaceID, b.ID, relationID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if deleted != 1 {
		t.Fatalf("deleted %d rows, want exactly one — one row is both directions", deleted)
	}

	for _, id := range []uuid.UUID{a.ID, b.ID} {
		if told[id] != entity.IssueActivityKindRelationRemoved {
			t.Errorf("issue %s was not told the relation went away", id)
		}
	}
}

func TestClosingADuplicateMovesOnlyTheDuplicate(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID := uuid.New(), uuid.New()
	dupe, survivor := issue("MOB", 7), issue("MOB", 3)
	dupe.TeamID = teamID

	canceled := entity.WorkflowState{
		ID: uuid.New(), TeamID: teamID, Name: "Canceled", Category: entity.StateCategoryAbandoned,
	}

	h.expectScope(workspaceID)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, dupe.ID, gomock.Any()).Return(dupe, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, survivor.ID, gomock.Any()).Return(survivor, nil)
	h.expectNoRelationHeld()
	h.relations.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r entity.StoredIssueRelation) (entity.StoredIssueRelation, error) {
			return r, nil
		})
	h.states.EXPECT().ListByTeamID(gomock.Any(), teamID).Return([]entity.WorkflowState{canceled}, nil)

	moved := map[uuid.UUID]uuid.UUID{}

	h.issues.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, id uuid.UUID, _ int, change entity.IssueChange, _ any, _ any,
		) error {
			moved[id] = *change.StateID

			return nil
		})

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).Times(3)

	if _, err := h.service.Add(context.Background(), workspaceID, dupe.ID, service.AddIssueRelationInput{
		Kind:           entity.IssueRelationViewDuplicates,
		CounterpartID:  survivor.ID,
		CloseDuplicate: true,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if moved[dupe.ID] != canceled.ID {
		t.Fatal("the duplicate was not closed")
	}

	if _, touched := moved[survivor.ID]; touched {
		t.Fatal(
			"the surviving issue was moved too. The direction of a duplicate relation says which " +
				"one survives, and closing both defeats the point of recording it.",
		)
	}
}

func TestRecordingADuplicateWithoutAskingLeavesBothIssuesAlone(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	dupe, survivor := issue("MOB", 7), issue("MOB", 3)

	h.expectScope(workspaceID)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, dupe.ID, gomock.Any()).Return(dupe, nil)
	h.issues.EXPECT().GetVisible(gomock.Any(), workspaceID, survivor.ID, gomock.Any()).Return(survivor, nil)
	h.expectNoRelationHeld()
	h.relations.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r entity.StoredIssueRelation) (entity.StoredIssueRelation, error) {
			return r, nil
		})
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	if _, err := h.service.Add(context.Background(), workspaceID, dupe.ID, service.AddIssueRelationInput{
		Kind:          entity.IssueRelationViewDuplicates,
		CounterpartID: survivor.ID,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
}

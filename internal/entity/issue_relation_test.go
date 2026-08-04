package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestEveryViewRoundTripsThroughItsStoredKindFromBothEnds(t *testing.T) {
	for _, view := range entity.IssueRelationViews() {
		kind, subjectIsSource := view.Stored()

		if !kind.Valid() {
			t.Errorf("%q stores as %q, which is not a kind the database accepts", view, kind)

			continue
		}

		if back := kind.As(subjectIsSource); back != view {
			t.Errorf(
				"%q stored as (%q, source=%v) reads back as %q. The inverse is computed on read, "+
					"so a view that does not survive the round trip is a relation that displays "+
					"as something other than what was asked for.",
				view, kind, subjectIsSource, back,
			)
		}
	}
}

func TestTheTwoEndsOfAStoredRelationReadAsEachOthersInverse(t *testing.T) {
	inverses := map[entity.IssueRelationKind][2]entity.IssueRelationView{
		entity.IssueRelationBlocks:     {entity.IssueRelationViewBlocks, entity.IssueRelationViewBlockedBy},
		entity.IssueRelationDuplicates: {entity.IssueRelationViewDuplicates, entity.IssueRelationViewDuplicatedBy},
		entity.IssueRelationRelatesTo:  {entity.IssueRelationViewRelatesTo, entity.IssueRelationViewRelatesTo},
	}

	for kind, want := range inverses {
		if got := kind.As(true); got != want[0] {
			t.Errorf("%q seen from the source reads %q, want %q", kind, got, want[0])
		}

		if got := kind.As(false); got != want[1] {
			t.Errorf("%q seen from the target reads %q, want %q", kind, got, want[1])
		}
	}
}

func TestOnlyRelatesToReadsTheSameFromBothEnds(t *testing.T) {
	for _, kind := range entity.IssueRelationKinds() {
		same := kind.As(true) == kind.As(false)

		if same != kind.Symmetric() {
			t.Errorf(
				"%q reads identically from both ends = %v, but Symmetric() = %v. A directional "+
					"relation that looks the same from either side cannot express which issue "+
					"blocks which.",
				kind, same, kind.Symmetric(),
			)
		}
	}
}

func TestNoViewStoresAnInverseKind(t *testing.T) {
	for _, view := range entity.IssueRelationViews() {
		kind, _ := view.Stored()

		switch kind {
		case entity.IssueRelationBlocks, entity.IssueRelationDuplicates, entity.IssueRelationRelatesTo:
		default:
			t.Errorf(
				"%q stores as %q. Only the three canonical kinds may reach the database; storing "+
					"an inverse would mean the same relation could exist in two forms.",
				view, kind,
			)
		}
	}
}

func TestAPairNormalisesToTheSameOrderWhicheverWayItIsGiven(t *testing.T) {
	a := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	b := uuid.MustParse("00000000-0000-4000-8000-0000000000ff")

	lowA, highA := entity.NormalisePair(a, b)
	lowB, highB := entity.NormalisePair(b, a)

	if lowA != lowB || highA != highB {
		t.Fatalf(
			"(%s, %s) normalised to (%s, %s) but the reverse gave (%s, %s). A symmetric relation "+
				"created from either side must land on one row.",
			a, b, lowA, highA, lowB, highB,
		)
	}

	if lowA != a || highA != b {
		t.Fatalf("normalised to (%s, %s), want the lower id first", lowA, highA)
	}
}

func TestNormalisingAnAlreadyOrderedPairChangesNothing(t *testing.T) {
	a := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	b := uuid.MustParse("00000000-0000-4000-8000-000000000002")

	low, high := entity.NormalisePair(a, b)
	again, andAgain := entity.NormalisePair(low, high)

	if again != low || andAgain != high {
		t.Fatal("normalising twice moved the pair; it must be idempotent")
	}
}

func TestRelationsAreGroupedInAStableOrderAndEmptyGroupsAreOmitted(t *testing.T) {
	relations := []entity.IssueRelation{
		{Kind: entity.IssueRelationViewRelatesTo, Issue: entity.Issue{ReferenceKey: "MOB", Number: 3}},
		{Kind: entity.IssueRelationViewBlockedBy, Issue: entity.Issue{ReferenceKey: "MOB", Number: 1}},
		{Kind: entity.IssueRelationViewRelatesTo, Issue: entity.Issue{ReferenceKey: "MOB", Number: 4}},
	}

	groups := entity.GroupRelations(relations)

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2 — a kind with no relations must not appear at all", len(groups))
	}

	if groups[0].Kind != entity.IssueRelationViewBlockedBy {
		t.Errorf("first group is %q; groups follow the declared view order, not insertion order", groups[0].Kind)
	}

	if len(groups[1].Relations) != 2 {
		t.Errorf("the relates-to group holds %d, want 2", len(groups[1].Relations))
	}
}

func TestGroupingNothingYieldsNoGroups(t *testing.T) {
	if groups := entity.GroupRelations(nil); len(groups) != 0 {
		t.Fatalf("got %d groups from no relations, want none", len(groups))
	}
}

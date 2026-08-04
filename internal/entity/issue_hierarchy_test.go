package entity_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestADepthCheckAccountsForTheWholeSubTreeNotJustTheIssueMoved(t *testing.T) {
	cases := map[string]struct {
		parentDepth   int
		subtreeHeight int
		want          bool
	}{
		"a leaf under the root":                    {1, 0, true},
		"a leaf at the deepest permitted place":    {entity.IssueMaxDepth - 1, 0, true},
		"a leaf one past the deepest place":        {entity.IssueMaxDepth, 0, false},
		"a two-tall sub-tree that just fits":       {2, 2, true},
		"a two-tall sub-tree one level too far":    {3, 2, false},
		"a sub-tree whose leaves would overshoot":  {3, 1, true},
		"the whole tree moved under another root":  {1, entity.IssueMaxDepth - 2, true},
		"a sub-tree taller than the tree can hold": {1, entity.IssueMaxDepth, false},
	}

	for name, tc := range cases {
		if got := entity.FitsWithinDepth(tc.parentDepth, tc.subtreeHeight); got != tc.want {
			t.Errorf(
				"%s: parent at depth %d taking a sub-tree %d tall = %v, want %v",
				name, tc.parentDepth, tc.subtreeHeight, got, tc.want,
			)
		}
	}
}

func TestMovingASubTreeIsRefusedEvenWhenTheIssueItselfWouldFit(t *testing.T) {
	parentDepth := entity.IssueMaxDepth - 2

	if !entity.FitsWithinDepth(parentDepth, 0) {
		t.Fatal("the issue alone does not fit, so this case proves nothing")
	}

	if entity.FitsWithinDepth(parentDepth, 2) {
		t.Fatalf(
			"a two-tall sub-tree was accepted under a depth-%d parent. Checking only where the "+
				"moved issue lands ignores its descendants, and they are what breaches the cap.",
			parentDepth,
		)
	}
}

func TestOnlyUnfinishedChildrenCountAsOpen(t *testing.T) {
	progress := entity.IssueProgress{NotStarted: 2, Active: 3, Complete: 7, Abandoned: 4}

	if got := entity.OpenChildren(progress); got != 5 {
		t.Fatalf(
			"open children = %d, want 5. A finished or abandoned child is not outstanding work "+
				"and must never hold its parent open.",
			got,
		)
	}
}

func TestAnAbandonedChildDoesNotHoldItsParentOpen(t *testing.T) {
	children := []entity.Issue{
		{State: entity.IssueState{Category: entity.StateCategoryComplete}},
		{State: entity.IssueState{Category: entity.StateCategoryAbandoned}},
	}

	if open := entity.OpenIssues(children); len(open) != 0 {
		t.Fatalf(
			"%d children reported open. An abandoned child is a decision not to do the work, "+
				"so it cannot block the parent from being finished.",
			len(open),
		)
	}
}

func TestOpenIssuesNamesEveryChildStillToDo(t *testing.T) {
	children := []entity.Issue{
		{ReferenceKey: "MOB", Number: 7, State: entity.IssueState{Category: entity.StateCategoryActive}},
		{ReferenceKey: "MOB", Number: 8, State: entity.IssueState{Category: entity.StateCategoryComplete}},
		{ReferenceKey: "PLT", Number: 2, State: entity.IssueState{Category: entity.StateCategoryNotStarted}},
	}

	open := entity.OpenIssues(children)

	if len(open) != 2 || open[0].Reference() != "MOB-7" || open[1].Reference() != "PLT-2" {
		t.Fatalf("open children = %v, want MOB-7 and PLT-2", open)
	}
}

func TestTheOpenChildrenRefusalNamesWhatIsBlocking(t *testing.T) {
	err := entity.IssueChildrenOpenError{
		Children: []entity.Issue{
			{ReferenceKey: "MOB", Number: 7},
			{ReferenceKey: "PLT", Number: 2},
		},
	}

	message := err.Error()

	for _, reference := range []string{"MOB-7", "PLT-2"} {
		if !strings.Contains(message, reference) {
			t.Errorf("%q does not name %q, so the caller cannot see what to close first", message, reference)
		}
	}
}

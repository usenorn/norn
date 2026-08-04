package entity_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestTwoActorsEditingDifferentFieldsBothLand(t *testing.T) {
	fieldVersions := map[string]int{entity.IssueFieldTitle: 8}

	conflicts := entity.IssueConflicts(fieldVersions, 8, 7, []string{entity.IssueFieldPriority})

	if len(conflicts) != 0 {
		t.Fatalf(
			"editing priority conflicted with %v after somebody else changed the title; "+
				"disjoint edits must both be reflected",
			conflicts,
		)
	}
}

func TestTwoActorsEditingTheSameFieldReportTheConflictByName(t *testing.T) {
	fieldVersions := map[string]int{entity.IssueFieldPriority: 8}

	conflicts := entity.IssueConflicts(fieldVersions, 8, 7, []string{entity.IssueFieldPriority})

	if !slices.Equal(conflicts, []string{entity.IssueFieldPriority}) {
		t.Fatalf("conflicts = %v, want [priority] so the client can say which field moved", conflicts)
	}
}

func TestTeamStateAndLabelsConflictWithEachOtherBecauseTheyAreNotIndependent(t *testing.T) {
	cases := map[string]struct {
		changed string
		touched string
	}{
		"a team move against a state change": {entity.IssueFieldTeam, entity.IssueFieldState},
		"a state change against a team move": {entity.IssueFieldState, entity.IssueFieldTeam},
		"labels against a team move":         {entity.IssueFieldLabels, entity.IssueFieldTeam},
		"a team move against labels":         {entity.IssueFieldTeam, entity.IssueFieldLabels},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			conflicts := entity.IssueConflicts(map[string]int{tc.changed: 8}, 8, 7, []string{tc.touched})

			if len(conflicts) == 0 {
				t.Fatalf(
					"changing %s while %s moved was allowed. These three are bound by foreign keys and "+
						"by the category remap, so field-level disjointness is not independence — "+
						"an issue could end up assigned to somebody who is not on the destination team.",
					tc.touched, tc.changed,
				)
			}
		})
	}
}

func TestAVersionThatAdvancedWithoutExplanationIsTreatedAsAConflict(t *testing.T) {
	conflicts := entity.IssueConflicts(map[string]int{}, 9, 7, []string{entity.IssueFieldTitle})

	if len(conflicts) == 0 {
		t.Fatal(
			"the row moved from version 7 to 9 but no field claims responsibility, and the write was " +
				"allowed anyway. A path that bumps the version without recording which field changed " +
				"must fail closed, or it silently discards the other actor's change.",
		)
	}
}

func TestAnUnchangedRowNeverConflicts(t *testing.T) {
	fieldVersions := map[string]int{entity.IssueFieldTitle: 3, entity.IssueFieldState: 5}

	for _, field := range entity.IssueFields() {
		if conflicts := entity.IssueConflicts(fieldVersions, 7, 7, []string{field}); len(conflicts) != 0 {
			t.Errorf("editing %s on an unchanged row conflicted with %v", field, conflicts)
		}
	}
}

func TestAStaleErrorCarriesTheCurrentVersionAndUnwrapsToTheSentinel(t *testing.T) {
	err := error(entity.IssueStaleError{Version: 9, Conflicts: []string{entity.IssueFieldTitle}})

	if !errors.Is(err, entity.ErrIssueStale) {
		t.Fatal("IssueStaleError does not unwrap to ErrIssueStale, so callers cannot detect it by identity")
	}

	var stale entity.IssueStaleError
	if !errors.As(err, &stale) || stale.Version != 9 {
		t.Fatal("IssueStaleError does not carry the version the client must re-read from")
	}
}

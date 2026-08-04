package entity_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyLabelsBelongingToAnotherTeamAreStrandedByAMove(t *testing.T) {
	here, elsewhere := uuid.New(), uuid.New()

	labels := []entity.Label{
		{Name: "Blocker"},
		{Name: "Platform debt", TeamID: here},
		{Name: "Design debt", TeamID: elsewhere},
	}

	stranded := entity.LabelsOutOfScope(labels, here)

	if len(stranded) != 1 || stranded[0].Name != "Design debt" {
		t.Fatalf(
			"stranded %v. A workspace-wide label crosses every team boundary and a label already "+
				"scoped to the destination is going home, so neither is lost.",
			names(stranded),
		)
	}
}

func TestAMoveIntoTheTeamALabelAlreadyBelongsToStrandsNothing(t *testing.T) {
	destination := uuid.New()

	stranded := entity.LabelsOutOfScope(
		[]entity.Label{{Name: "Platform debt", TeamID: destination}}, destination,
	)

	if len(stranded) != 0 {
		t.Fatalf("refused a move that loses nothing: %v", names(stranded))
	}
}

func TestTheCounterpartStateIsTheEarliestOneSharingTheCategory(t *testing.T) {
	states := []entity.WorkflowState{
		{ID: uuid.New(), Name: "Done", Category: entity.StateCategoryComplete, Position: 5},
		{ID: uuid.New(), Name: "In review", Category: entity.StateCategoryActive, Position: 4},
		{ID: uuid.New(), Name: "In progress", Category: entity.StateCategoryActive, Position: 3},
	}

	counterpart, found := entity.CounterpartState(states, entity.StateCategoryActive)

	if !found || counterpart.Name != "In progress" {
		t.Fatalf(
			"picked %q. A team with several states in one category must resolve to the earliest, "+
				"so a move lands at the start of the destination's workflow rather than part-way through.",
			counterpart.Name,
		)
	}
}

func TestATeamWithNoStateInTheCategoryReportsNoCounterpart(t *testing.T) {
	states := []entity.WorkflowState{
		{ID: uuid.New(), Name: "Todo", Category: entity.StateCategoryNotStarted, Position: 1},
	}

	if _, found := entity.CounterpartState(states, entity.StateCategoryComplete); found {
		t.Fatal(
			"reported a counterpart that does not exist; the caller would move the issue to a " +
				"zero state id and the state foreign key would reject the whole transaction",
		)
	}
}

func TestTheOutOfScopeRefusalNamesEveryLabelThatWouldBeLost(t *testing.T) {
	err := entity.IssueLabelsOutOfScopeError{
		Labels: []entity.Label{{Name: "Design debt"}, {Name: "Needs spec"}},
	}

	message := err.Error()

	for _, name := range []string{"Design debt", "Needs spec"} {
		if !strings.Contains(message, name) {
			t.Errorf("%q does not name %q, so the caller cannot say what acknowledging costs", message, name)
		}
	}
}

func names(labels []entity.Label) []string {
	out := make([]string, 0, len(labels))

	for _, label := range labels {
		out = append(out, label.Name)
	}

	return out
}

func TestAReferenceIsReadTheWayPeopleWriteIt(t *testing.T) {
	cases := map[string]entity.IssueReference{
		"MOB-14":   {Key: "MOB", Number: 14},
		"mob-14":   {Key: "MOB", Number: 14},
		"  DES-1 ": {Key: "DES", Number: 1},
		"AB-999":   {Key: "AB", Number: 999},
	}

	for raw, want := range cases {
		got, err := entity.ParseIssueReference(raw)
		if err != nil {
			t.Errorf("%q: %v", raw, err)

			continue
		}

		if got != want {
			t.Errorf("%q parsed to %+v, want %+v", raw, got, want)
		}
	}
}

func TestAReferenceThatCouldNeverHaveBeenIssuedIsRefused(t *testing.T) {
	for _, raw := range []string{
		"", "MOB", "14", "MOB-", "-14", "MOB-0", "MOB--1", "TOOLONG-1", "M-1", "MOB-1.5", "MOB-abc",
	} {
		if _, err := entity.ParseIssueReference(raw); err == nil {
			t.Errorf("%q was accepted as a reference", raw)
		}
	}
}

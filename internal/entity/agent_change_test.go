package entity_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAChangeNamesOnlyWhatItActuallyAsksFor(t *testing.T) {
	title := "The export button does nothing"
	priority := entity.IssuePriorityHigh

	asked := entity.AgentChange{
		Title:    &title,
		Priority: &priority,
		Clear:    []string{"estimate"},
	}.Parts()

	want := []entity.ChangePart{
		entity.ChangePartTitle,
		entity.ChangePartPriority,
		entity.ChangePartEstimate,
	}

	if len(asked) != len(want) {
		t.Fatalf("the change names %v, want %v", asked, want)
	}

	for _, part := range want {
		if !slices.Contains(asked, part) {
			t.Fatalf("the change does not name %q, though it asks for it", part)
		}
	}
}

func TestAcceptingSomePartsLeavesTheRestUnchanged(t *testing.T) {
	title := "A better title"
	described := "A description that misses the point"
	assignee := uuid.New()

	narrowed := entity.AgentChange{
		ExpectedVersion: 7,
		Title:           &title,
		Description:     &described,
		AssigneeID:      &assignee,
		Clear:           []string{"dueOn", "estimate"},
	}.Only([]entity.ChangePart{entity.ChangePartTitle, entity.ChangePartDueOn})

	switch {
	case narrowed.Title == nil || *narrowed.Title != title:
		t.Fatalf("the accepted title did not survive: %+v", narrowed.Title)
	case narrowed.Description != nil:
		t.Fatalf(
			"a description nobody accepted was still applied. Accepting a title must not carry "+
				"text the approver read and refused: %q",
			*narrowed.Description,
		)
	case narrowed.AssigneeID != nil:
		t.Fatalf("an assignee nobody accepted was still applied")
	case len(narrowed.Clear) != 1 || narrowed.Clear[0] != "dueOn":
		t.Fatalf("the accepted clears came out as %v, want the due date alone", narrowed.Clear)
	case narrowed.ExpectedVersion != 7:
		t.Fatalf("narrowing lost the version the change was written against")
	}
}

func TestAcceptingNothingInParticularAcceptsTheWholeChange(t *testing.T) {
	title := "A better title"
	described := "And a better description"

	whole := entity.AgentChange{Title: &title, Description: &described}
	narrowed := whole.Only(nil)

	if narrowed.Title == nil || narrowed.Description == nil {
		t.Fatalf(
			"naming no parts dropped some of the change. An approver who did not choose meant " +
				"all of it, which is what approving used to mean.",
		)
	}
}

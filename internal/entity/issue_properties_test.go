package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

var (
	entered = time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	moved   = time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
)

func TestMovingBetweenTwoStatesOfTheSameCategoryDoesNotRestartTheClock(t *testing.T) {
	before := entity.ApplyStateTransition(
		entity.StateCategoryActive, entity.StateCategoryActive, entered, moved,
	)

	if !before.StateEnteredAt.Equal(entered) {
		t.Fatalf(
			"state_entered_at moved to %s. An issue that goes from one team's \"In progress\" to "+
				"another's has been in progress continuously; only the row's identity changed.",
			before.StateEnteredAt,
		)
	}
}

func TestEnteringACompleteStateStampsCompletionAndLeavingItClearsCompletion(t *testing.T) {
	done := entity.ApplyStateTransition(
		entity.StateCategoryActive, entity.StateCategoryComplete, entered, moved,
	)

	if done.CompletedAt == nil || !done.CompletedAt.Equal(moved) {
		t.Fatal("entering a complete-category state did not stamp completed_at")
	}

	reopened := entity.ApplyStateTransition(
		entity.StateCategoryComplete, entity.StateCategoryActive, entered, moved,
	)

	if reopened.CompletedAt != nil {
		t.Fatal("reopening an issue left completed_at set")
	}
}

func TestReassigningACompletedIssueToAnAbandonedStateClearsCompletion(t *testing.T) {
	abandoned := entity.ApplyStateTransition(
		entity.StateCategoryComplete, entity.StateCategoryAbandoned, entered, moved,
	)

	if abandoned.CompletedAt != nil {
		t.Fatal(
			"deleting a \"Done\" state and reassigning its issues to \"Canceled\" left completed_at " +
				"set on an abandoned issue — the issue was never finished",
		)
	}

	if !abandoned.StateEnteredAt.Equal(moved) {
		t.Fatal("a genuine category change must restart state_entered_at")
	}
}

func TestStayingInACompleteCategoryKeepsTheOriginalCompletionMoment(t *testing.T) {
	shuffled := entity.ApplyStateTransition(
		entity.StateCategoryComplete, entity.StateCategoryComplete, entered, moved,
	)

	if shuffled.CompletedAt == nil || !shuffled.CompletedAt.Equal(entered) {
		t.Fatal(
			"moving between two complete-category states rewrote when the issue was finished; " +
				"it was finished when it first entered the category",
		)
	}
}

func TestOnlyTheFivePrioritiesAreValid(t *testing.T) {
	for _, priority := range entity.IssuePriorities() {
		if !priority.Valid() {
			t.Errorf("%q is offered but not valid", priority)
		}
	}

	for _, priority := range []entity.IssuePriority{"", "critical", "blocker", "Urgent", "P0"} {
		if priority.Valid() {
			t.Errorf("%q is accepted but the CHECK constraint would reject it", priority)
		}
	}
}

func TestADescriptionMayNotCarryTheOneByteThatBreaksTheRoundTrip(t *testing.T) {
	if got := entity.ValidateIssueDescription("description", "fine\x00text").Code; got != entity.ValidationCodeMalformed {
		t.Fatalf(
			"a NUL byte was accepted (code %q). Postgres text cannot store it, so the description "+
				"would not round-trip unchanged.",
			got,
		)
	}

	markdown := "# Title\n\n```go\nfunc main() {}\n```\n\n- a\r\n- b\t\n"
	if got := entity.ValidateIssueDescription("description", markdown).Code; got != "" {
		t.Fatalf("ordinary markdown with a code block was rejected: %q", got)
	}
}

func TestDescriptionLengthIsCountedInRunesNotBytes(t *testing.T) {
	wide := strings.Repeat("é", entity.IssueDescriptionMaxLen)

	if got := entity.ValidateIssueDescription("description", wide).Code; got != "" {
		t.Fatalf(
			"a description of exactly the limit in characters was rejected as %q — it is being "+
				"measured in bytes, so non-English text gets half the allowance",
			got,
		)
	}
}

func TestIssueTitleLengthIsCountedInRunesNotBytes(t *testing.T) {
	wide := strings.Repeat("é", entity.IssueTitleMaxLen)

	if got := entity.ValidateIssueTitle("title", wide).Code; got != "" {
		t.Fatalf(
			"a title of exactly the limit in characters was rejected as %q — every neighbouring "+
				"validator counts runes, and this one counted bytes",
			got,
		)
	}
}

func TestAnEstimateIsAPositivePointCount(t *testing.T) {
	cases := map[int]string{
		0:                           entity.ValidationCodeUnsupportedValue,
		-3:                          entity.ValidationCodeUnsupportedValue,
		entity.IssueEstimateMax + 1: entity.ValidationCodeUnsupportedValue,
		1:                           "",
		13:                          "",
		entity.IssueEstimateMax:     "",
	}

	for estimate, want := range cases {
		if got := entity.ValidateIssueEstimate("estimate", estimate).Code; got != want {
			t.Errorf("estimate %d gave %q, want %q", estimate, got, want)
		}
	}
}

package entity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyTheThreeVerdictsANormalRunCanReportAreValid(t *testing.T) {
	valid := map[entity.ValidationStatus]bool{
		entity.ValidationPassed:  true,
		entity.ValidationFailed:  true,
		entity.ValidationSkipped: true,
		"":                       false,
		"pass":                   false,
		"errored":                false,
		"probably fine":          false,
	}

	for status, want := range valid {
		if status.Valid() != want {
			t.Errorf(
				"%q reports Valid() as %t; the column has a CHECK on exactly these three, so "+
					"anything else fails the write instead of the message",
				status, !want,
			)
		}
	}
}

func TestAChangeWithNoRepositoryIsRefusedBecauseNothingCouldBeKeyedOnIt(t *testing.T) {
	err := entity.ValidateExecutionChange("repos[0]", entity.ExecutionChange{Branch: "main"})

	var invalid entity.ValidationError

	if !errors.As(err, &invalid) {
		t.Fatalf("a change naming no repository was accepted, answering %v", err)
	}

	if invalid.Fields[0].Field != "repos[0].repo" ||
		invalid.Fields[0].Code != entity.ValidationCodeRequired {
		t.Fatalf("the refusal points at %+v, which does not name the missing field", invalid.Fields)
	}
}

func TestACountThatWentBackwardsIsRefused(t *testing.T) {
	err := entity.ValidateExecutionChange("repos[0]", entity.ExecutionChange{
		Repository: "backend",
		Deletions:  -12,
	})

	var invalid entity.ValidationError

	if !errors.As(err, &invalid) {
		t.Fatalf(
			"a negative diffstat was accepted, answering %v; the column refuses it, so the write "+
				"would fail the whole message instead",
			err,
		)
	}
}

func TestABranchNameLongerThanTheColumnIsRefusedBeforeItReachesIt(t *testing.T) {
	err := entity.ValidateExecutionChange("repos[0]", entity.ExecutionChange{
		Repository: "backend",
		Branch:     strings.Repeat("b", entity.CodebaseBranchMaxLen+1),
	})

	var invalid entity.ValidationError

	if !errors.As(err, &invalid) {
		t.Fatalf("an over-long branch name was accepted, answering %v", err)
	}
}

func TestAValidationResultNeedsSomethingToCallTheCheck(t *testing.T) {
	err := entity.ValidateExecutionValidation("validation[0]", entity.ExecutionValidation{
		Status: entity.ValidationPassed,
	})

	var invalid entity.ValidationError

	if !errors.As(err, &invalid) {
		t.Fatalf(
			"a validation result with no name was accepted, answering %v; the row is keyed on "+
				"the name, so an empty one would collide with the next unnamed check",
			err,
		)
	}
}

func TestAChangeSetNothingHasReportedIntoIsEmpty(t *testing.T) {
	if !(entity.ExecutionChangeSet{}).Empty() {
		t.Fatal(
			"a run that has reported nothing does not read as empty, so the API would hand a " +
				"review screen a result made of zeroes",
		)
	}

	reported := entity.ExecutionChangeSet{
		Changes: []entity.ExecutionChange{{Repository: "backend"}},
	}

	if reported.Empty() {
		t.Fatal("a changeset carrying a repository reads as empty")
	}
}

func TestAPublicationIsOnlyCompleteWhenNoRepositoryIsStillGoingOrFailed(t *testing.T) {
	published := func(states ...entity.PublicationState) []entity.RepositoryPublication {
		repositories := make([]entity.RepositoryPublication, 0, len(states))
		for _, state := range states {
			repositories = append(repositories,
				entity.RepositoryPublication{Publication: entity.ExecutionPublication{State: state}})
		}

		return repositories
	}

	for name, tc := range map[string]struct {
		repositories []entity.RepositoryPublication
		want         entity.PublicationOutcome
	}{
		"all published": {
			published(entity.PublicationPublished, entity.PublicationPushed), entity.PublicationOutcomeComplete,
		},
		"one failed": {
			published(entity.PublicationPublished, entity.PublicationFailed), entity.PublicationOutcomeIncomplete,
		},
		"one still going": {
			published(entity.PublicationFailed, entity.PublicationPending), entity.PublicationOutcomeUnderway,
		},
	} {
		if got := entity.PublicationOutcomeOf(tc.repositories); got != tc.want {
			t.Errorf("%s: outcome %d, want %d", name, got, tc.want)
		}
	}
}

func TestAPublicationReportIsCurrentOnlyWhenEveryRepositoryHoldsItsRevisionAndAttempt(t *testing.T) {
	held := func(revision, attempt int) entity.ExecutionChange {
		return entity.ExecutionChange{Publication: entity.ExecutionPublication{Revision: revision, Attempt: attempt}}
	}

	for name, tc := range map[string]struct {
		changes []entity.ExecutionChange
		want    bool
	}{
		"every repository took it":            {[]entity.ExecutionChange{held(2, 2), held(2, 2)}, true},
		"a newer attempt kept one repository": {[]entity.ExecutionChange{held(2, 2), held(2, 3)}, false},
		"nothing was stored":                  {nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := (entity.ExecutionChangeSet{Changes: tc.changes}).Holds(2, 2); got != tc.want {
				t.Fatalf("Holds = %v, want %v", got, tc.want)
			}
		})
	}
}

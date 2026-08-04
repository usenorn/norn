package entity_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestEveryDomainErrorABulkLoopCanMeetClassifiesToAnOutcome(t *testing.T) {
	cases := map[error]entity.BulkOutcome{
		nil:                                 entity.BulkOutcomeApplied,
		entity.ErrIssueNotFound:             entity.BulkOutcomeNotFound,
		entity.ErrWorkflowStateNotFound:     entity.BulkOutcomeNotFound,
		entity.ErrLabelNotFound:             entity.BulkOutcomeNotFound,
		entity.ErrTeamNotFound:              entity.BulkOutcomeNotFound,
		entity.ErrAccountForbidden:          entity.BulkOutcomeForbidden,
		entity.ErrIssueStale:                entity.BulkOutcomeConflict,
		entity.ErrIssueStatusTransition:     entity.BulkOutcomeConflict,
		entity.ErrIssueChildrenOpen:         entity.BulkOutcomeConflict,
		entity.ErrLabelOutOfScope:           entity.BulkOutcomeInvalid,
		entity.ErrIssueDestinationIncapable: entity.BulkOutcomeInvalid,
	}

	for err, want := range cases {
		if got := entity.OutcomeFor(err); got != want {
			t.Errorf("OutcomeFor(%v) = %q, want %q", err, got, want)
		}
	}
}

func TestAWrappedDomainErrorStillClassifies(t *testing.T) {
	wrapped := fmt.Errorf("apply state to MOB-4: %w", entity.ErrIssueStale)

	if got := entity.OutcomeFor(wrapped); got != entity.BulkOutcomeConflict {
		t.Fatalf(
			"a wrapped stale error classified as %q. Repository and service layers wrap as they "+
				"return, so matching only bare sentinels would file most real failures as invalid.",
			got,
		)
	}
}

func TestATypedStaleErrorIsAConflictNotAValidationFailure(t *testing.T) {
	stale := entity.IssueStaleError{Version: 4, Conflicts: []string{"state"}}

	if got := entity.OutcomeFor(stale); got != entity.BulkOutcomeConflict {
		t.Fatalf(
			"a typed IssueStaleError classified as %q. It unwraps to ErrIssueStale, and telling a "+
				"user their change was invalid when someone simply edited first is misleading.",
			got,
		)
	}
}

func TestAValidationErrorIsInvalidRatherThanASilentSuccess(t *testing.T) {
	invalid := entity.NewValidationError(entity.FieldError{
		Field: "priority",
		Code:  entity.ValidationCodeUnsupportedValue,
	})

	if got := entity.OutcomeFor(invalid); got != entity.BulkOutcomeInvalid {
		t.Fatalf("a validation error classified as %q, want invalid", got)
	}
}

func TestAnUnrecognisedFailureIsNeverReportedAsApplied(t *testing.T) {
	if got := entity.OutcomeFor(errors.New("the database fell over")); got.Applied() {
		t.Fatalf(
			"an unknown error classified as %q, which Applied() treats as success. An unclassified "+
				"failure must never be reported to the user as a change that landed.",
			got,
		)
	}
}

func TestEveryOrderedPairOfBulkStatusesIsDecidedDeliberately(t *testing.T) {
	permitted := map[entity.BulkActionStatus][]entity.BulkActionStatus{
		entity.BulkActionQueued:   {entity.BulkActionRunning, entity.BulkActionFailed},
		entity.BulkActionRunning:  {entity.BulkActionComplete, entity.BulkActionFailed},
		entity.BulkActionComplete: {},
		entity.BulkActionFailed:   {},
	}

	for _, from := range entity.BulkActionStatuses() {
		for _, to := range entity.BulkActionStatuses() {
			want := false

			for _, allowed := range permitted[from] {
				if allowed == to {
					want = true
				}
			}

			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s -> %s is %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestASettledBulkActionNeverStartsAgain(t *testing.T) {
	for _, settled := range []entity.BulkActionStatus{entity.BulkActionComplete, entity.BulkActionFailed} {
		if !settled.Settled() {
			t.Errorf("%s does not report as settled", settled)
		}

		if settled.CanTransitionTo(entity.BulkActionRunning) {
			t.Errorf(
				"%s can start running again. A redelivered task would re-apply every change and "+
					"write a second set of activity rows.",
				settled,
			)
		}
	}
}

func TestAChangeMustAlterSomething(t *testing.T) {
	if err := (entity.BulkChange{}).Validate(); !errors.Is(err, entity.ErrBulkChangeEmpty) {
		t.Fatalf("an empty change validated as %v, want a refusal", err)
	}
}

func TestArchivingAndEditingInOneChangeIsRefused(t *testing.T) {
	archived := entity.IssueStatusArchived
	urgent := entity.IssuePriorityUrgent

	change := entity.BulkChange{Status: &archived, Priority: &urgent}

	if err := change.Validate(); !errors.Is(err, entity.ErrBulkChangeConflicting) {
		t.Fatalf(
			"archiving and reprioritising in one action validated as %v. The two take different "+
				"paths through the issue service, and silently doing one of them is worse than refusing.",
			err,
		)
	}
}

func TestSettingAndClearingTheAssigneeAtOnceIsRefused(t *testing.T) {
	someone := uuid.New()
	change := entity.BulkChange{AssigneeID: &someone, ClearAssignee: true}

	if err := change.Validate(); !errors.Is(err, entity.ErrBulkChangeConflicting) {
		t.Fatalf("setting and clearing the assignee together validated as %v", err)
	}
}

func TestASetIsEitherIdentifiersOrAFilterButNeverBoth(t *testing.T) {
	if err := (entity.BulkSet{}).Validate(); !errors.Is(err, entity.ErrBulkSetEmpty) {
		t.Error("an empty set was accepted")
	}

	both := entity.BulkSet{IssueIDs: []uuid.UUID{uuid.New()}, Filter: &entity.BulkFilter{}}
	if err := both.Validate(); !errors.Is(err, entity.ErrBulkSetAmbiguous) {
		t.Error("a set carrying both identifiers and a filter was accepted")
	}
}

func TestOnlyABoundedIdentifierSetRunsInline(t *testing.T) {
	ids := make([]uuid.UUID, entity.BulkSyncLimit)
	for i := range ids {
		ids[i] = uuid.New()
	}

	if !(entity.BulkSet{IssueIDs: ids}).RunsInline() {
		t.Errorf("a set of exactly %d does not run inline; the limit is inclusive", entity.BulkSyncLimit)
	}

	if (entity.BulkSet{IssueIDs: append(ids, uuid.New())}).RunsInline() {
		t.Errorf("a set of %d runs inline, one past the limit", entity.BulkSyncLimit+1)
	}

	if (entity.BulkSet{Filter: &entity.BulkFilter{}}).RunsInline() {
		t.Fatal(
			"a filter-defined set runs inline. Its size is unknown until it is walked, which is " +
				"exactly why it must become a job.",
		)
	}
}

func TestAFilterSetReportsNoExpectedSizeSoProgressStaysIndeterminate(t *testing.T) {
	if expected := (entity.BulkSet{Filter: &entity.BulkFilter{}}).Expected(); expected != nil {
		t.Fatalf(
			"a filter set claims to expect %d issues. Knowing that up front means counting them, "+
				"which is the one tally this product refuses to compute.",
			*expected,
		)
	}

	ids := []uuid.UUID{uuid.New(), uuid.New()}
	if expected := (entity.BulkSet{IssueIDs: ids}).Expected(); expected == nil || *expected != 2 {
		t.Fatal("an identifier set must report the size the caller itself supplied")
	}
}

func TestWorkIsSplitIntoChunksThatCoverEveryItemExactlyOnce(t *testing.T) {
	items := make([]int, 57)
	for i := range items {
		items[i] = i
	}

	chunks := entity.Chunks(items, entity.BulkChunkSize)

	seen := map[int]int{}

	for _, chunk := range chunks {
		if len(chunk) > entity.BulkChunkSize {
			t.Fatalf("a chunk holds %d items, beyond the chunk size", len(chunk))
		}

		for _, item := range chunk {
			seen[item]++
		}
	}

	if len(seen) != len(items) {
		t.Fatalf("chunking covered %d of %d items", len(seen), len(items))
	}

	for item, times := range seen {
		if times != 1 {
			t.Fatalf("item %d appears in %d chunks; it would be changed twice", item, times)
		}
	}
}

func TestChunkingNothingProducesNoChunks(t *testing.T) {
	if chunks := entity.Chunks([]int{}, entity.BulkChunkSize); len(chunks) != 0 {
		t.Fatalf("got %d chunks from no items", len(chunks))
	}
}

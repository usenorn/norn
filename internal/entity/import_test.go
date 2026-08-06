package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnImportCanAlwaysBeAbandonedButNeverResurrected(t *testing.T) {
	for _, status := range entity.ImportStatuses() {
		reachable := status.CanTransitionTo(entity.ImportFailed)

		switch status {
		case entity.ImportFailed, entity.ImportReverted:
			if reachable {
				t.Errorf("%q can fail again. A settled import has nothing left to abandon.", status)
			}
		default:
			if !reachable {
				t.Errorf(
					"%q cannot fail. Every phase of an import can be interrupted by something "+
						"outside it, and a run with nowhere to go records nothing about why.",
					status,
				)
			}
		}
	}

	if entity.ImportReverted.CanTransitionTo(entity.ImportReverting) {
		t.Error("a reverted import can be reverted again, which would walk a ledger it already emptied")
	}
}

func TestOnlyAFinishedImportCanBeReverted(t *testing.T) {
	for _, status := range entity.ImportStatuses() {
		wanted := status == entity.ImportImported || status == entity.ImportFailed

		if status.Revertible() != wanted {
			t.Errorf("%q reports revertible = %v, want %v", status, status.Revertible(), wanted)
		}

		if status == entity.ImportReverting {
			continue
		}

		if status.CanTransitionTo(entity.ImportReverting) != wanted {
			t.Errorf("%q can move to reverting = %v, want %v", status, !wanted, wanted)
		}
	}

	if !entity.ImportReverting.CanTransitionTo(entity.ImportReverting) {
		t.Error(
			"a revert cannot continue into its next slice, so a walk longer than one lease " +
				"would have nowhere to go",
		)
	}
}

func TestAPartlyAppliedImportIsStillRevertible(t *testing.T) {
	if !entity.ImportFailed.Revertible() {
		t.Fatal(
			"a failed import cannot be reverted. A run that stopped halfway has already created " +
				"things and recorded every one of them, and the ledger exists precisely so that " +
				"half an import can be taken back.",
		)
	}
}

func TestOnlyTheThreeWorkingStatusesHoldALease(t *testing.T) {
	working := map[entity.ImportStatus]bool{
		entity.ImportStaging:   true,
		entity.ImportExecuting: true,
		entity.ImportReverting: true,
	}

	for _, status := range entity.ImportStatuses() {
		if status.Leasable() != working[status] {
			t.Errorf(
				"%q reports leasable = %v. Only a status with a worker behind it may be rescued; "+
					"a run parked at mapped is waiting for a person and must be left alone.",
				status, status.Leasable(),
			)
		}
	}
}

func TestMappingAndPreviewCanBeRepeatedWithoutMovingOn(t *testing.T) {
	if !entity.ImportStaged.CanTransitionTo(entity.ImportStaged) {
		t.Error("a staged import cannot stay staged, so editing a mapping would advance the run")
	}

	if !entity.ImportMapped.CanTransitionTo(entity.ImportStaged) {
		t.Error("a mapped import cannot go back to staged, so a withdrawn decision would be stuck")
	}
}

func TestEveryImportPhaseIsADistinctResourceInDependencyOrder(t *testing.T) {
	seen := make(map[entity.ImportResource]int, len(entity.ImportPhases()))

	for position, resource := range entity.ImportPhases() {
		if previous, repeated := seen[resource]; repeated {
			t.Errorf("%q appears at positions %d and %d", resource, previous, position)
		}

		seen[resource] = position

		if !resource.Valid() {
			t.Errorf("%q is a phase but not a valid resource", resource)
		}
	}

	for _, ordering := range [][2]entity.ImportResource{
		{entity.ImportTeam, entity.ImportWorkflowState},
		{entity.ImportTeam, entity.ImportProject},
		{entity.ImportLabelGroup, entity.ImportLabel},
		{entity.ImportWorkflowState, entity.ImportIssue},
		{entity.ImportProject, entity.ImportIssue},
		{entity.ImportIssue, entity.ImportIssueParent},
		{entity.ImportIssue, entity.ImportIssueRelation},
		{entity.ImportIssue, entity.ImportComment},
	} {
		if seen[ordering[0]] >= seen[ordering[1]] {
			t.Errorf(
				"%q is imported at or after %q, which depends on it",
				ordering[0], ordering[1],
			)
		}
	}
}

func TestIssueLinksAreTheirOwnPhaseSoARevertCanUnpickThemFirst(t *testing.T) {
	if entity.ImportIssueParent.Fetched() {
		t.Error(
			"a parent link is fetched from the source as a resource of its own. Every tracker " +
				"models the parent as a property of the child, so it already arrives on the issue " +
				"record; asking a source for the same link a second time doubles the traffic and " +
				"lets a page and its links disagree.",
		)
	}

	if !entity.ImportIssueRelation.Fetched() {
		t.Error(
			"a relation is not fetched, so nothing can ever stage one. A relation is symmetric " +
				"and belongs to neither of the issues it joins, so unlike a parent it has no " +
				"record to ride in on; a source with no relations answers with an empty page.",
		)
	}

	phases := entity.ImportPhases()

	var issue, parent int

	for position, resource := range phases {
		switch resource {
		case entity.ImportIssue:
			issue = position
		case entity.ImportIssueParent:
			parent = position
		}
	}

	if parent <= issue {
		t.Fatal(
			"parent links are recorded before the issues they join. workspace_issues clears a " +
				"parent on delete and then refuses the row for having depth without one, so a " +
				"revert walking the ledger backwards must reach every link before any issue.",
		)
	}
}

func TestAnImportRunRemembersEnoughToAuthorizeItselfLater(t *testing.T) {
	account := uuid.New()

	run := entity.ImportRun{
		RequestedByAccount:  account,
		RequestedActorKind:  entity.ActorKindUser,
		RequestedAuthMethod: entity.SessionAuthMethodSSO,
	}

	requester := run.Requester()

	if requester.AccountID != account {
		t.Errorf("requester account = %v, want %v", requester.AccountID, account)
	}

	if requester.AuthMethod != entity.SessionAuthMethodSSO {
		t.Fatalf(
			"requester auth method = %q, want sso. A workspace enforcing single sign-on refuses "+
				"an actor with no method, so a queued import that forgot how its requester signed "+
				"in would be denied on every row it touched.",
			requester.AuthMethod,
		)
	}
}

func TestARevertIsAuthorizedAsWhoeverAsksForItNow(t *testing.T) {
	importer, reverter := uuid.New(), uuid.New()

	run := entity.ImportRun{
		RequestedByAccount: importer,
		RequestedActorKind: entity.ActorKindToken,
		RevertedByAccount:  reverter,
		RevertedActorKind:  entity.ActorKindUser,
		RevertedAuthMethod: entity.SessionAuthMethodPassword,
	}

	if run.Reverter().AccountID != reverter {
		t.Fatalf(
			"reverter account = %v, want %v. An import may be undone months later by somebody "+
				"else, and the account that ran it may have left the workspace since.",
			run.Reverter().AccountID, reverter,
		)
	}
}

func TestAMissingActorKindReadsAsAPerson(t *testing.T) {
	run := entity.ImportRun{RequestedByAccount: uuid.New()}

	if run.Requester().Kind != entity.ActorKindUser {
		t.Errorf("requester kind = %q, want user", run.Requester().Kind)
	}
}

func TestARefusedRowIsSkippedAndABrokenOneFails(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		outcome entity.ImportOutcome
	}{
		{"nothing went wrong", nil, entity.ImportOutcomeCreated},
		{"the team is not visible", entity.ErrTeamNotFound, entity.ImportOutcomeSkipped},
		{"the label was deleted", entity.ErrLabelNotFound, entity.ImportOutcomeSkipped},
		{"the account may not", entity.ErrAccountForbidden, entity.ImportOutcomeSkipped},
		{
			"the row is malformed",
			entity.NewValidationError(entity.FieldError{Field: "title", Code: entity.ValidationCodeRequired}),
			entity.ImportOutcomeSkipped,
		},
		{"the database is down", errors.New("connection refused"), entity.ImportOutcomeFailed},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if outcome := entity.OutcomeForImport(testCase.err); outcome != testCase.outcome {
				t.Errorf("outcome = %q, want %q", outcome, testCase.outcome)
			}
		})
	}
}

func TestEveryOutcomeAndStateIsInItsOwnList(t *testing.T) {
	for _, outcome := range entity.ImportOutcomes() {
		if !outcome.Valid() {
			t.Errorf("%q is listed but not valid", outcome)
		}
	}

	if entity.ImportOutcome("invented").Valid() {
		t.Error("an outcome nobody declared reports itself valid")
	}

	for _, state := range entity.ImportRecordStates() {
		if !state.Valid() {
			t.Errorf("%q is listed but not valid", state)
		}
	}

	for _, phase := range entity.ImportPhaseNames() {
		if !phase.Valid() {
			t.Errorf("%q is listed but not valid", phase)
		}
	}
}

func TestOnlyADeletionOrAnArchiveCountsAsUndone(t *testing.T) {
	undone := map[entity.ImportOutcome]bool{
		entity.ImportOutcomeDeleted:  true,
		entity.ImportOutcomeArchived: true,
	}

	for _, outcome := range entity.ImportOutcomes() {
		if outcome.Reverted() != undone[outcome] {
			t.Errorf(
				"%q reports reverted = %v. A row that was skipped because somebody had edited it "+
					"is still there, and a revert that claimed otherwise would be lying.",
				outcome, outcome.Reverted(),
			)
		}
	}
}

func TestAParkedCursorIsOnlyParkedUntilItsTimeArrives(t *testing.T) {
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	soon := now.Add(time.Minute)
	past := now.Add(-time.Minute)

	if (entity.ImportCursor{}).Parked(now) {
		t.Error("a cursor with no retry time reports itself parked, so staging would never resume")
	}

	if !(entity.ImportCursor{RetryAfter: &soon}).Parked(now) {
		t.Error("a cursor asked to wait another minute is not parked")
	}

	if (entity.ImportCursor{RetryAfter: &past}).Parked(now) {
		t.Error("a cursor whose wait has passed is still parked, so a rescue would skip it forever")
	}
}

func TestABackoffIsBoundedAtBothEnds(t *testing.T) {
	floor, ceiling := time.Second, 15*time.Minute

	cases := []struct {
		name      string
		requested time.Duration
		want      time.Duration
	}{
		{"a source asking for nothing still waits", 0, floor},
		{"a source asking for less than the floor waits the floor", time.Millisecond, floor},
		{"a reasonable request is obeyed", 30 * time.Second, 30 * time.Second},
		{"a source asking for four hours is asked again at the ceiling", 4 * time.Hour, ceiling},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := entity.ClampImportBackoff(testCase.requested, floor, ceiling); got != testCase.want {
				t.Errorf("backoff = %s, want %s", got, testCase.want)
			}
		})
	}
}

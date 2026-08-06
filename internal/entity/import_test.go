package entity_test

import (
	"errors"
	"strings"
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
		{entity.ImportTeam, entity.ImportCycle},
		{entity.ImportCycle, entity.ImportIssue},
		{entity.ImportIssue, entity.ImportIssueParent},
		{entity.ImportIssue, entity.ImportIssueRelation},
		{entity.ImportIssue, entity.ImportComment},
		{entity.ImportIssue, entity.ImportAttachment},
		{entity.ImportComment, entity.ImportAttachment},
		{entity.ImportAttachment, entity.ImportEmbed},
		{entity.ImportComment, entity.ImportEmbed},
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

func TestAnInlineImageIsDerivedFromTheFileRecordRatherThanAskedForAgain(t *testing.T) {
	if entity.ImportEmbed.Fetched() {
		t.Error(
			"an embed is fetched from the source as a resource of its own. The file record " +
				"already says which body points at it, so asking a source to describe the same " +
				"link a second time doubles the traffic and lets a page and its rewrites disagree.",
		)
	}

	if !entity.ImportAttachment.Fetched() {
		t.Error(
			"a file is not fetched, so nothing can ever stage one. The bytes are pulled while " +
				"the source's signed URL is alive, which only the staging pass is in a position " +
				"to do.",
		)
	}
}

func TestAnEmbedMarkerSurvivesBeingWrittenIntoAMarkdownLink(t *testing.T) {
	marker := entity.ImportEmbedMarker("att-7")

	body := "Before\n\n![The hub](" + marker + ")\n\nAfter"

	if !entity.ImportEmbedded(body) {
		t.Fatal("a body carrying a marker does not read as carrying one")
	}

	rewritten := strings.ReplaceAll(body, marker, "/v1/whatever")

	if entity.ImportEmbedded(rewritten) {
		t.Fatalf(
			"replacing the marker left %q still reading as embedded. The marker is the whole of "+
				"what an adapter puts in a body in place of a signed URL, so anything the rewrite "+
				"cannot take back out is a link that expires in the middle of somebody's issue.",
			rewritten,
		)
	}

	if strings.Contains(marker, " ") || strings.Contains(marker, ")") {
		t.Errorf(
			"the marker %q contains a character that ends a markdown link, so the link it stands "+
				"in for would break the moment an adapter wrote it",
			marker,
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

func TestWhatASourceLegitimatelyHoldsCostsTheRowRatherThanTheRun(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		why     string
		wrapped error
	}{
		{
			name: "two issues already hold a relation",
			err:  entity.ErrIssueRelationExists,
			why: "Norn allows one relation per pair in either direction and other trackers " +
				"allow several, so a mature workspace produces this on ordinary data",
		},
		{
			name: "an issue relates to itself",
			err:  entity.ErrIssueRelationSelf,
			why:  "an export can carry a self-relation that its own product tolerated",
		},
		{
			name: "two projects slugify the same",
			err:  entity.ErrProjectSlugTaken,
			why:  "distinct source names can collapse onto one address here",
		},
		{
			name: "an imported cycle overlaps one the team already generated",
			err:  entity.ErrCycleOverlaps,
			why: "a team that already runs cycles here will overlap its own history, and " +
				"the exclusion constraint is the database saying so",
		},
		{
			name: "a cycle belongs to another team",
			err:  entity.ErrCycleTeamMismatch,
			why:  "a source can reference a cycle across the team boundary Norn draws",
		},
		{
			name:    "the workspace ran out of storage",
			err:     entity.ErrStorageExhausted,
			wrapped: entity.StorageExhaustedError{SizeBytes: 1 << 20, StoredBytes: 9, MaxBytes: 10},
			why: "rows are unbounded and bytes are not, so a full workspace must cost the " +
				"file and keep the backlog",
		},
		{
			name: "the import may not edit a comment it attributed to somebody else",
			err:  entity.ErrIssueCommentNotAuthor,
			why: "only an author edits their own comment here, and an import writes comments as " +
				"whoever the source said wrote them, so a body it wanted to rewrite is one it is " +
				"routinely refused",
		},
		{
			name:    "one file is larger than this instance accepts",
			err:     entity.ErrAttachmentTooLarge,
			wrapped: entity.AttachmentTooLargeError{SizeBytes: 1 << 30, MaxBytes: 1 << 20},
			why:     "one oversized attachment says nothing about the three years around it",
		},
		{
			name: "the workspace already holds a team with that key",
			err:  entity.ErrTeamKeyTaken,
			why: "a second import of the same source, and any import into a workspace somebody " +
				"has already set up, arrives at the keys that are there",
		},
		{
			name: "the team already holds a state with that name",
			err:  entity.ErrWorkflowStateNameTaken,
			why: "every tracker ships Todo, In progress and Done, so a team that exists here at " +
				"all collides with the source's own workflow",
		},
		{
			name: "the workspace already holds a label with that name",
			err:  entity.ErrLabelNameTaken,
			why: "labels are the shortest names in the product and the ones most likely to have " +
				"been created already, by hand or by an earlier run of this same import",
		},
		{
			name: "the workspace already holds a label group with that name",
			err:  entity.ErrLabelGroupNameTaken,
			why:  "a group is created before the labels in it and collides for the same reasons",
		},
		{
			name: "an issue carries two labels from one group",
			err:  entity.ErrLabelGroupExclusive,
			why: "grouped labels are exclusive here and not everywhere, so a source can carry a " +
				"row wearing two of them",
		},
		{
			name: "the reference an issue was given is already used",
			err:  entity.ErrIssueReferenceTaken,
			why: "the team's counter is shared with everybody working in it while the import " +
				"runs, and one row losing that race is not the backlog behind it going wrong",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if outcome := entity.OutcomeForImport(testCase.err); outcome != entity.ImportOutcomeSkipped {
				t.Errorf(
					"outcome = %q, want skipped. This is not a broken import, it is %s. "+
						"Recorded as failed it settles the record failed and, once the run has "+
						"retried its way to the attempt limit, abandons everything after it.",
					outcome, testCase.why,
				)
			}

			if testCase.wrapped == nil {
				return
			}

			if outcome := entity.OutcomeForImport(testCase.wrapped); outcome != entity.ImportOutcomeSkipped {
				t.Errorf(
					"the detailed error carrying the sizes is %q rather than skipped, so the "+
						"outcome would depend on which of the two forms happened to reach it",
					outcome,
				)
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

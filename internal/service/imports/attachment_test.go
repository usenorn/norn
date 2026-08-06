package imports_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const hubDescription = "It sags in the middle"

func illustratedSource(t *testing.T) *staticSource {
	t.Helper()

	return &staticSource{
		kind: "illustrated",
		held: map[entity.ImportResource][]entity.ImportRecord{
			entity.ImportIssue: {
				fetched(t, sourceIssueHub, "", service.ImportIssuePayload{
					Title: "Rework the hub",
					Description: hubDescription + "\n\n![The hub](" +
						entity.ImportEmbedMarker(sourceShot) + ")",
					Team: sourceTeam,
				}),
			},
			entity.ImportAttachment: {
				fetched(t, sourceShot, "", service.ImportAttachmentPayload{
					Issue: sourceIssueHub, FileName: "hub-shot.png", ContentType: "image/png",
					SizeBytes: 24_812, ObjectKey: sourceShotKey, SourceURL: sourceShotURL,
					Inline: true,
				}),
			},
		},
	}
}

func illustrated(t *testing.T, h *harness, team entity.Team) *applied {
	t.Helper()

	h.offering(illustratedSource(t))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	onlyTheTeamIsDecided(h, team.ID)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	return made
}

func executing(t *testing.T, h *harness) {
	t.Helper()

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}
}

func TestAFileTheSourceHeldArrivesAsAnAdoptedObjectRatherThanAFetch(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := illustrated(t, h, team)

	executing(t, h)

	if len(made.files) != 1 {
		t.Fatalf("the import adopted %d files, want the one the source held", len(made.files))
	}

	adopted := made.files[0]

	if adopted.ObjectKey != sourceShotKey {
		t.Errorf(
			"the file was adopted from %q, want the key staging stored it under (%q). The bytes "+
				"were pulled while the source's signed URL was alive; the apply pass has nothing "+
				"left to fetch and no live URL to fetch it from.",
			adopted.ObjectKey, sourceShotKey,
		)
	}

	if adopted.Origin == nil || !adopted.Origin.Attributed() {
		t.Fatalf(
			"the file was adopted without an attributed origin (%#v). Adopt is reserved for an "+
				"import by exactly that test, so an inert origin is refused outright and the file "+
				"never arrives.",
			adopted.Origin,
		)
	}

	entries := h.ledgerFor(entity.ImportAttachment)

	if len(entries) != 1 {
		t.Fatalf("the ledger holds %d files, want the one that was adopted", len(entries))
	}

	if entries[0].Reference != sourceShotKey {
		t.Errorf(
			"the ledger names the file %q rather than the object key it was stored under. A revert "+
				"months later has the ledger and nothing else to say which object in the bucket "+
				"this import put there.",
			entries[0].Reference,
		)
	}
}

func TestAWorkspaceThatFillsUpLosesTheFileAndKeepsTheBacklog(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := illustrated(t, h, team)
	made.refuse = entity.StorageExhaustedError{SizeBytes: 24_812, StoredBytes: 99, MaxBytes: 100}

	because := "Rows are unbounded and bytes are not. A quota that costs the chunk rather than " +
		"the file turns one screenshot over the line into every issue behind it abandoned: the " +
		"slice is retried, fills up again at the same file, and settles the whole run failed."

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("a full workspace ended the slice: %v. %s", err, because)
	}

	if h.run().Status != entity.ImportImported {
		t.Fatalf("a full workspace left the run at %q. %s", h.run().Status, because)
	}

	if state := h.recordState(entity.ImportAttachment, sourceShot); state != entity.ImportRecordSkipped {
		t.Errorf(
			"the file reads as %q, want skipped. Recorded failed it would settle the run failed "+
				"once the attempt limit ran out.",
			state,
		)
	}

	line, found := h.lineOf(entity.ImportAttachment, sourceShot)

	if !found || line.Outcome != entity.ImportOutcomeSkipped {
		t.Fatalf("the report says %q about the file that would not fit, want skipped", line.Outcome)
	}

	if !strings.Contains(line.Subject, "hub-shot.png") {
		t.Errorf(
			"the report names the lost file %q. The requester is the only person who can go and "+
				"raise the quota or fetch that one file by hand, and they can only do it if the "+
				"report says which file it was.",
			line.Subject,
		)
	}

	if len(h.ledgerFor(entity.ImportIssue)) != 1 {
		t.Errorf("the issue the file hung off was not imported, so the backlog went with the file")
	}
}

func TestAFileHangingOffAnIssueThatNeverArrivedIsSkippedRatherThanFatal(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)
	h.stage(entity.ImportAttachment, fetched(t, sourceSpec, "", service.ImportAttachmentPayload{
		Issue: "issue-never-exported", FileName: "hub-spec.pdf",
		ContentType: "application/pdf", SizeBytes: 91_003, ObjectKey: sourceSpecKey,
		SourceURL: sourceSpecURL,
	}))
	onlyTheTeamIsDecided(h, team.ID)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf(
			"a file naming an issue the export left behind ended the run: %v. A partial export is "+
				"the normal case, and a file with nowhere to hang is one line of the report.",
			err,
		)
	}

	if len(made.files) != 0 {
		t.Errorf("the orphaned file was adopted anyway, so it hangs off nothing at all")
	}

	line, found := h.lineOf(entity.ImportAttachment, sourceSpec)

	if !found || line.Outcome != entity.ImportOutcomeSkipped {
		t.Fatalf("the report says %q about the orphaned file, want skipped", line.Outcome)
	}

	if h.run().Status != entity.ImportImported {
		t.Errorf("run ended %q, want imported", h.run().Status)
	}
}

func TestAnInlineImageEndsUpPointingAtTheFileThisWorkspaceStored(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := illustrated(t, h, team)

	executing(t, h)

	hub := made.issueTitled(t, "Rework the hub")
	files := h.ledgerFor(entity.ImportAttachment)

	if len(files) != 1 {
		t.Fatalf("the ledger holds %d files, want the one the description points at", len(files))
	}

	wanted := entity.AttachmentContentPath(h.run().WorkspaceID, files[0].CreatedID)

	if !strings.Contains(hub.Description, wanted) {
		t.Fatalf(
			"the description reads %q and never points at %q. The picture is stored either way; "+
				"a body left pointing at the source is a page that renders today and shows a "+
				"broken image the moment the export's signed URL expires.",
			hub.Description, wanted,
		)
	}

	if entity.ImportEmbedded(hub.Description) {
		t.Errorf(
			"the description still carries the marker: %q. The marker is scaffolding an adapter "+
				"writes in place of a URL it cannot keep, and one left behind is rendered to the "+
				"reader as literal text.",
			hub.Description,
		)
	}

	if !strings.Contains(hub.Description, hubDescription) {
		t.Errorf(
			"rewriting the link lost the rest of the description (%q). Only the marker is the "+
				"import's to replace.",
			hub.Description,
		)
	}

	if len(h.ledgerFor(entity.ImportEmbed)) != 1 {
		t.Errorf("the rewrite never reached the ledger, so a revert has no record that it happened")
	}
}

func TestABodyWhoseFileCouldNotBeStoredIsLeftAloneAndSaidSo(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := illustrated(t, h, team)
	made.refuse = entity.StorageExhaustedError{SizeBytes: 24_812, StoredBytes: 99, MaxBytes: 100}

	executing(t, h)

	hub := made.issueTitled(t, "Rework the hub")

	if !entity.ImportEmbedded(hub.Description) {
		t.Fatalf(
			"the description was rewritten to %q although the file it points at was never stored. "+
				"Whatever path it now names answers with nothing, and the reader is left with a "+
				"broken image where the source at least had a URL somebody could still open.",
			hub.Description,
		)
	}

	if slices.ContainsFunc(h.trace, func(step string) bool {
		return strings.HasPrefix(step, "rewrite ")
	}) {
		t.Errorf("the import edited a body it had nothing to put in it: %v", h.trace)
	}

	line, found := h.lineOf(entity.ImportEmbed, sourceIssueHub)

	if !found || line.Outcome != entity.ImportOutcomeSkipped {
		t.Fatalf(
			"the report says %q about the rewrite that could not be made. A description quietly "+
				"left pointing at somebody else's expiring URL is the sort of thing noticed "+
				"months later by whoever opens the issue.",
			line.Outcome,
		)
	}

	if !strings.Contains(line.Detail["reason"], "not stored") {
		t.Errorf("the report explains the skip as %q, want the file named as unstored", line.Detail["reason"])
	}
}

func TestRevertingTakesTheFileBackOutAndLeavesTheRewriteToTheRowItWasWrittenOn(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := illustrated(t, h, team)

	executing(t, h)

	hub := made.issueTitled(t, "Rework the hub")

	h.trace = nil
	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if len(made.stored) != 0 {
		t.Fatalf(
			"reverting left %d files standing. Storage is metered against the workspace, so a "+
				"revert that gives back the rows and keeps the bytes leaves the quota spent on an "+
				"import that no longer exists.",
			len(made.stored),
		)
	}

	if !slices.Contains(h.trace, "discard hub-shot.png") {
		t.Errorf("the file was never discarded: %v", h.trace)
	}

	if outcome := outcomeOfEntry(t, h, entity.ImportEmbed, hub.ID); outcome != entity.ImportOutcomeRetained {
		t.Fatalf(
			"the rewrite was recorded as %q on the way out. An embed made nothing of its own — it "+
				"edited a description the issue entry beside it deletes outright — so anything but "+
				"retained is a revert claiming to have undone something it never touched.",
			outcome,
		)
	}

	if h.run().Status != entity.ImportReverted {
		t.Errorf("run ended %q, want reverted", h.run().Status)
	}
}

func TestARewriteTheImportMadeItselfIsNotMistakenForSomebodyElsesEdit(t *testing.T) {
	cases := []struct {
		name    string
		edited  bool
		outcome entity.ImportOutcome
		because string
	}{
		{
			name:    "nobody but the import touched it",
			outcome: entity.ImportOutcomeDeleted,
			because: "The embed phase rewrote this description to point at the file it had just " +
				"stored, which moved the version past what the issue phase recorded. Read " +
				"literally that is an edit, and a revert that believes it refuses to remove " +
				"every illustrated issue it ever created — the requester is told the import was " +
				"undone and finds most of it still there.",
		},
		{
			name:    "somebody worked on it afterwards",
			edited:  true,
			outcome: entity.ImportOutcomeSkippedModified,
			because: "Restamping the ledger at the rewrite must not blind the check to a person " +
				"who came along afterwards; deleting their work is the one failure a revert " +
				"cannot apologise for.",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t).backed()
			team := teamNamed("Core")

			h.scopedTo(team.ID)

			made := illustrated(t, h, team)

			executing(t, h)

			hub := made.issueTitled(t, "Rework the hub")

			if testCase.edited {
				editedSince(made, hub)
			}

			h.trace = nil
			h.at(entity.ImportImported)

			if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
				t.Fatalf("run revert: %v", err)
			}

			if outcome := outcomeOfEntry(t, h, entity.ImportIssue, hub.ID); outcome != testCase.outcome {
				t.Fatalf(
					"the issue was recorded as %q on the way out, want %q. %s",
					outcome, testCase.outcome, testCase.because,
				)
			}

			_, standing := made.held[hub.ID]

			if standing == (testCase.outcome == entity.ImportOutcomeDeleted) {
				t.Errorf("the ledger says %q but the issue standing = %v", testCase.outcome, standing)
			}
		})
	}
}

func TestACommentTheImportMayNotEditKeepsItsBodyRatherThanFailingTheRun(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue, fetched(t, sourceIssueHub, "", service.ImportIssuePayload{
		Title: "Rework the hub", Team: sourceTeam,
	}))
	h.stage(entity.ImportComment, fetched(t, sourceComment, "", service.ImportCommentPayload{
		Issue: sourceIssueHub, Body: "Look: ![shot](" + entity.ImportEmbedMarker(sourceShot) + ")",
		Author: ida,
	}))
	h.stage(entity.ImportAttachment, fetched(t, sourceShot, "", service.ImportAttachmentPayload{
		Issue: sourceIssueHub, Comment: sourceComment, FileName: "hub-shot.png",
		ContentType: "image/png", SizeBytes: 24_812, ObjectKey: sourceShotKey,
		SourceURL: sourceShotURL, Inline: true,
	}))
	h.stage(entity.ImportEmbed, entity.ImportRecord{
		ExternalID: sourceComment,
		Payload: payloadOf(t, service.ImportEmbedPayload{
			Issue: sourceIssueHub, Comment: sourceComment, Attachments: []string{sourceShot},
		}),
	})
	h.decided(
		entity.ImportMapping{
			Kind: entity.ImportMapTeam, SourceKey: sourceTeam,
			Decision: entity.ImportDecisionMap, TargetID: team.ID,
		},
		entity.ImportMapping{
			Kind: entity.ImportMapUser, SourceKey: ida.Key,
			Decision: entity.ImportDecisionMap, TargetID: uuid.New(),
		},
	)

	applying(h, team)

	h.at(entity.ImportMapped)

	executing(t, h)

	if h.run().Status != entity.ImportImported {
		t.Fatalf(
			"the run ended %q. A comment is written as whoever the source said wrote it, and only "+
				"an author may edit their own comment here — so a rewrite the import is refused "+
				"is the ordinary case, not a broken import.",
			h.run().Status,
		)
	}

	line, found := h.lineOf(entity.ImportEmbed, sourceComment)

	if !found || line.Outcome != entity.ImportOutcomeSkipped {
		t.Fatalf(
			"the report says %q about a rewrite the workspace refused, want it skipped and named",
			line.Outcome,
		)
	}
}

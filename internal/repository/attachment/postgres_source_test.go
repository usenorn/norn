package attachment

import (
	"regexp"
	"strings"
	"testing"
)

var statements = map[string]string{
	"createAttachmentQuery":          createAttachmentQuery,
	"attachmentByIDQuery":            attachmentByIDQuery,
	"attachmentsByIssueQuery":        attachmentsByIssueQuery,
	"lockStoredAttachmentByKeyQuery": lockStoredAttachmentByKeyQuery,
	"settleAttachmentQuery":          settleAttachmentQuery,
	"discardAttachmentQuery":         discardAttachmentQuery,
	"claimForCommentQuery":           claimForCommentQuery,
	"markOrphansQuery":               markOrphansQuery,
	"reclaimableQuery":               reclaimableQuery,
	"reclaimAttachmentQuery":         reclaimAttachmentQuery,
	"admitStorageQuery":              admitStorageQuery,
	"releaseStorageQuery":            releaseStorageQuery,
	"correctStorageQuery":            correctStorageQuery,
	"ledgerQuery":                    ledgerQuery,
	"claimImportFileQuery":           claimImportFileQuery,
	"lockImportFileQuery":            lockImportFileQuery,
	"recordImportFileQuery":          recordImportFileQuery,
	"unsettledImportFilesQuery":      unsettledImportFilesQuery,
	"takeImportFileQuery":            takeImportFileQuery,
	"unsettledAttachmentsQuery":      unsettledAttachmentsQuery,
	"measureAttachmentQuery":         measureAttachmentQuery,
	"refundImportFileQuery":          refundImportFileQuery,
}

func TestNoAttachmentQueryAggregatesAnything(t *testing.T) {
	counting := regexp.MustCompile(`(?i)\b(count|sum|avg|array_agg)\s*\(`)

	for name, query := range statements {
		if counting.MatchString(query) {
			t.Errorf(
				"%s aggregates rows. Consumption is a stored fact this process adds up from its "+
					"own writes, never a tally taken across a scope the reader cannot enumerate — "+
					"which is what a workspace-wide sum would be.",
				name,
			)
		}
	}
}

func TestEveryDeleteAlsoMovesTheLedger(t *testing.T) {
	for name, query := range statements {
		if !strings.Contains(query, "DELETE FROM workspace_issue_attachments") {
			continue
		}

		if !strings.Contains(query, "workspace_storage_ledger") {
			t.Errorf(
				"%s removes an attachment row without touching the ledger, so its bytes stay on "+
					"the bill forever. Deleting and decrementing are one statement precisely so "+
					"that forgetting is unwritable.",
				name,
			)
		}
	}
}

func TestTheAdmissionRefusesTheFirstUploadIntoAnEmptyLedger(t *testing.T) {
	insertion, update, found := strings.Cut(admitStorageQuery, "ON CONFLICT")
	if !found {
		t.Fatal("the admission is no longer an upsert")
	}

	if !strings.Contains(insertion, "$2::bigint <= $3::bigint") {
		t.Fatal(
			"the insert branch of the admission does not check the cap. The first upload into a " +
				"workspace takes that branch, where the ON CONFLICT predicate never runs, so an " +
				"over-limit file would walk straight through on an empty ledger.",
		)
	}

	if !strings.Contains(update, "stored_bytes + $2::bigint") ||
		!strings.Contains(update, "<= coalesce(workspace_storage_ledger.max_bytes, $3::bigint)") {
		t.Fatal("the update branch of the admission does not check the cap")
	}
}

func TestAWorkspacesOwnLimitWinsOverTheInstanceDefault(t *testing.T) {
	_, update, _ := strings.Cut(admitStorageQuery, "ON CONFLICT")

	if strings.Count(update, "coalesce(workspace_storage_ledger.max_bytes, $3::bigint)") != 2 {
		t.Fatal(
			"the admission does not read the workspace's own limit on both sides of its check. " +
				"A workspace given more room would be refused at the instance default, and one " +
				"given unlimited room would still be capped.",
		)
	}

	if !strings.Contains(ledgerQuery, "coalesce(l.max_bytes, $2::bigint)") {
		t.Fatal(
			"the ledger reports the instance default rather than the workspace's own limit, so " +
				"the settings page would disagree with what an upload is actually held to",
		)
	}
}

func TestDiscardingZeroesTheBytesItGaveBackSoTheSweepCannotGiveThemBackAgain(t *testing.T) {
	if !strings.Contains(discardAttachmentQuery, "size_bytes = 0") {
		t.Fatal(
			"a discarded row keeps its size after its bytes were released. The sweep subtracts " +
				"size_bytes when it deletes the row, so every removed file would be taken off the " +
				"ledger twice and the workspace would read emptier than it is.",
		)
	}

	if !strings.Contains(discardAttachmentQuery, "size_bytes = $3") {
		t.Fatal(
			"the discard does not require the row to still hold the bytes that were released. " +
				"A row resized in between would have one size given back and another forgotten.",
		)
	}
}

func TestReleasingIsNeverGatedByTheLimit(t *testing.T) {
	for _, name := range []string{"releaseStorageQuery", "refundImportFileQuery", "correctStorageQuery"} {
		if strings.Contains(statements[name], "<=") {
			t.Errorf(
				"%s is conditional on the cap. A workspace that is already over its limit could "+
					"then never free anything or be put back to what it really stores, which is "+
					"the one state it has to escape.",
				name,
			)
		}
	}
}

func TestNothingCanDriveTheLedgerNegative(t *testing.T) {
	for _, name := range []string{
		"reclaimAttachmentQuery", "releaseStorageQuery", "refundImportFileQuery", "correctStorageQuery",
	} {
		if !strings.Contains(statements[name], "greatest(") {
			t.Errorf(
				"%s subtracts without a floor. Drift after a crash between the object delete and "+
					"the commit would otherwise produce a negative invoice rather than a small "+
					"over-bill.",
				name,
			)
		}
	}
}

func TestAnImportFileIsOnlyEverReachedThroughItsOwnWorkspace(t *testing.T) {
	for _, name := range []string{
		"lockImportFileQuery", "recordImportFileQuery", "takeImportFileQuery", "refundImportFileQuery",
	} {
		if !strings.Contains(statements[name], "workspace_id = $2::uuid") {
			t.Errorf(
				"%s finds an import file by its key alone. A key is a string a caller hands in, "+
					"and without the workspace one import could move another workspace's bytes.",
				name,
			)
		}
	}
}

func TestTheSweepOnlyMeasuresImportFilesWhoseWritersHaveHadTheirChance(t *testing.T) {
	if !strings.Contains(unsettledImportFilesQuery, "settle_after <= $1") {
		t.Fatal(
			"the sweep picks import files without waiting for their settle deadline. A file still " +
				"being written would be measured as missing and refunded mid-upload.",
		)
	}
}

func TestAMeasurementNeverResizesAFileThatWasRemoved(t *testing.T) {
	for _, name := range []string{"unsettledAttachmentsQuery", "measureAttachmentQuery", "lockStoredAttachmentByKeyQuery"} {
		if !strings.Contains(statements[name], "status = 'stored'") {
			t.Errorf(
				"%s reaches attachments that are not stored. A removed file already gave its bytes "+
					"back and carries a size of zero; measuring it would put its object back on the "+
					"ledger just before the sweep deletes it.",
				name,
			)
		}
	}
}

func TestAClaimedImportFileIsLockedBeforeItsSizeIsRead(t *testing.T) {
	if !strings.Contains(lockImportFileQuery, "FOR UPDATE") {
		t.Fatal(
			"the size an import file already holds is read without a lock. Two uploads under the " +
				"same name would both charge the whole file instead of one charging the difference.",
		)
	}
}

func TestASettleOnlyEverLandsOnSomethingStillPending(t *testing.T) {
	if !strings.Contains(settleAttachmentQuery, "status = 'pending'") {
		t.Fatal(
			"the settle write does not require the attachment to still be pending. A double " +
				"finalize would then admit the same bytes twice.",
		)
	}
}

func TestACommentOnlyEverClaimsUnclaimedAttachmentsOnItsOwnIssue(t *testing.T) {
	for _, predicate := range []string{"workspace_id = $1", "issue_id = $2", "comment_id IS NULL"} {
		if !strings.Contains(claimForCommentQuery, predicate) {
			t.Fatalf(
				"the claim is missing %q. Without it a comment could adopt a file from another "+
					"issue, and the file's permissions would then disagree with where it appears.",
				predicate,
			)
		}
	}
}

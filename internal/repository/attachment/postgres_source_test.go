package attachment

import (
	"regexp"
	"strings"
	"testing"
)

var statements = map[string]string{
	"createAttachmentQuery":   createAttachmentQuery,
	"attachmentByIDQuery":     attachmentByIDQuery,
	"attachmentsByIssueQuery": attachmentsByIssueQuery,
	"settleAttachmentQuery":   settleAttachmentQuery,
	"discardAttachmentQuery":  discardAttachmentQuery,
	"claimForCommentQuery":    claimForCommentQuery,
	"markOrphansQuery":        markOrphansQuery,
	"reclaimableQuery":        reclaimableQuery,
	"reclaimAttachmentQuery":  reclaimAttachmentQuery,
	"admitStorageQuery":       admitStorageQuery,
	"releaseStorageQuery":     releaseStorageQuery,
	"ledgerQuery":             ledgerQuery,
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

	if !strings.Contains(update, "stored_bytes + $2::bigint <= $3::bigint") {
		t.Fatal("the update branch of the admission does not check the cap")
	}
}

func TestReleasingIsNeverGatedByTheLimit(t *testing.T) {
	if strings.Contains(releaseStorageQuery, "<=") {
		t.Fatal(
			"releasing bytes is conditional on the cap. A workspace that is already over its " +
				"limit could then never free anything, which is the one state it has to escape.",
		)
	}
}

func TestNothingCanDriveTheLedgerNegative(t *testing.T) {
	for _, name := range []string{"reclaimAttachmentQuery", "releaseStorageQuery"} {
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

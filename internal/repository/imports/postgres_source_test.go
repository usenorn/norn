package imports

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func packageSource(t *testing.T) string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading this package: %v", err)
	}

	var combined strings.Builder

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "mock_") {
			continue
		}

		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		combined.Write(body)
	}

	if combined.Len() == 0 {
		t.Fatal("this guard found no source, so it is protecting nothing")
	}

	return combined.String()
}

func TestNoImportQueryCountsWhatItIsCarrying(t *testing.T) {
	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)

	if counting.MatchString(packageSource(t)) {
		t.Fatal(
			"an import query tallies rows. Import volume is unbounded by design — a source with " +
				"four hundred thousand issues is a supported import — so nothing here may scan the " +
				"records table to say how big the job is. Progress is reported over work already " +
				"staged, one chunk at a time.",
		)
	}
}

func TestNoImportQueryReachesOutsideTheImportTables(t *testing.T) {
	naming := regexp.MustCompile(`(?m)\b(?:FROM|INTO|JOIN)\s+([A-Za-z_][A-Za-z0-9_]*)|^UPDATE\s+([A-Za-z_][A-Za-z0-9_]*)`)

	found := 0

	for _, match := range naming.FindAllStringSubmatch(packageSource(t), -1) {
		table := match[1]
		if table == "" {
			table = match[2]
		}

		found++

		if !strings.HasPrefix(table, "workspace_import_") {
			t.Errorf(
				"an import query names %q. This repository stages what a source handed over and "+
					"nothing else; the moment it reads or writes a product table directly, an import "+
					"is creating work outside the ledger that reverting cannot undo.",
				table,
			)
		}
	}

	if found == 0 {
		t.Fatal("no query names a table at all, so this guard is protecting nothing")
	}
}

func TestEveryLimitIsBoundRatherThanWrittenIn(t *testing.T) {
	limits := regexp.MustCompile(`\bLIMIT\s+(\S+)`).FindAllStringSubmatch(packageSource(t), -1)

	if len(limits) == 0 {
		t.Fatal("no query limits anything, so this guard is protecting nothing")
	}

	for _, limit := range limits {
		if !strings.HasPrefix(limit[1], "$") {
			t.Errorf(
				"a query limits itself to %q. Every chunk size is the caller's to choose and the "+
					"caller's to tune under load; a number written into the SQL is a chunk size "+
					"nobody can change.",
				limit[1],
			)
		}
	}
}

func TestClaimingRescuesARunWhoseWorkerDied(t *testing.T) {
	if !strings.Contains(claimRunQuery, "lease_expires_at <") {
		t.Error(
			"claiming matches only on the status it is moving from. A worker that died mid-import " +
				"leaves the run in that status forever, and every later attempt is refused as " +
				"leased elsewhere — the import strands with no way back short of a hand-written " +
				"update.",
		)
	}

	if !strings.Contains(claimRunQuery, "attempt = attempt + 1") {
		t.Error(
			"claiming does not count the attempt, so a run that fails and is rescued forever " +
				"looks identical to one being worked on for the first time",
		)
	}
}

func TestTheHeartbeatIsFencedByTheLeaseToken(t *testing.T) {
	if !strings.Contains(heartbeatRunQuery, "lease_token = ") {
		t.Fatal(
			"the heartbeat is not guarded by the token. A worker that stalled long enough to lose " +
				"its lease would extend the lease of whoever rescued the run, and two workers " +
				"would then import the same source twice.",
		)
	}
}

func TestAReleasedRunCanBePickedUpImmediately(t *testing.T) {
	if !strings.Contains(releaseRunQuery, "lease_expires_at = NULL") {
		t.Error(
			"releasing leaves an expiry behind. An import runs in slices: the worker releases at " +
				"the end of its budget and re-enqueues itself, and the next slice claims on the " +
				"expiry having passed. An expiry set to the release instant is not yet past, so " +
				"the continuation is refused and the import stalls until the lease ages out.",
		)
	}

	if !strings.Contains(releaseRunQuery, "lease_token = $2") {
		t.Error(
			"releasing is not fenced by the token, so a worker that already lost its lease would " +
				"clear the lease of the worker that took the run over",
		)
	}
}

func TestARunThatReleasedItsLeaseAndWasThenForgottenIsStillRescued(t *testing.T) {
	if !strings.Contains(leaseExpiredQuery, "lease_expires_at IS NULL") {
		t.Error(
			"the rescue sweep looks only for an expiry that has passed. Between finishing a slice " +
				"and its own re-enqueue a run holds no lease at all, so its expiry is NULL; if that " +
				"enqueue is lost the run sits in executing forever and an expiry test can never see " +
				"it. Resuming a half-finished import is the whole reason this sweep exists.",
		)
	}

	if !strings.Contains(leaseExpiredQuery, "updated_at <") {
		t.Error(
			"the sweep matches a run holding no lease without asking how long it has been that " +
				"way, so it would rescue a run whose re-enqueue is still in flight and put two " +
				"workers on the same chunk",
		)
	}

	if !strings.Contains(leaseExpiredQuery, "FOR UPDATE SKIP LOCKED") {
		t.Error(
			"the sweep does not skip locked rows, so two workers running it at once would both " +
				"rescue the same run",
		)
	}
}

func TestARecordIsOnlySettledWhileItIsStillStaged(t *testing.T) {
	if !strings.Contains(settleRecordsQuery, "state = 'staged'") {
		t.Error(
			"settling a record does not require it to still be staged. A redelivered chunk would " +
				"then move records a previous run of that same chunk already applied, and the " +
				"progress it reports would count them a second time.",
		)
	}

	if !strings.Contains(settleRecordsQuery, "RETURNING") {
		t.Error(
			"settling returns nothing, so the caller cannot tell which records it actually moved " +
				"and has no honest number to advance progress by",
		)
	}
}

func TestRevertWalksTheLedgerBackwardsOverWhatStandsUnreverted(t *testing.T) {
	if !strings.Contains(walkLedgerQuery, "order_seq DESC") {
		t.Error(
			"the ledger walk does not run backwards. Reverting has to undo what was created in " +
				"the reverse of the order it was created, or it deletes a parent while its " +
				"children still point at it.",
		)
	}

	if !strings.Contains(walkLedgerQuery, "reverted_at IS NULL") {
		t.Error(
			"the ledger walk returns entries that were already reverted, so a resumed revert " +
				"tries to delete rows it has already deleted",
		)
	}
}

func TestNoStatementIsIssuedOncePerRow(t *testing.T) {
	loops := regexp.MustCompile("(?s)\n\tfor [^\n]*\\{\n(.*?)\n\t\\}").
		FindAllStringSubmatch(packageSource(t), -1)

	if len(loops) == 0 {
		t.Fatal("this guard found no loops, so it is protecting nothing")
	}

	for _, loop := range loops {
		for _, call := range []string{"ExecContext", "QueryContext"} {
			if strings.Contains(loop[1], call) {
				t.Errorf(
					"a loop in this package calls %s. A page of staged records is one statement "+
						"over unnested arrays; a statement per row turns an import of four hundred "+
						"thousand issues into four hundred thousand round trips.",
					call,
				)
			}
		}
	}
}

func TestEveryInsertWritesEveryColumnItNames(t *testing.T) {
	source := packageSource(t)

	inserts := regexp.MustCompile("(?s)INSERT INTO [a-z_]+ \\((.*?)\\)\n(?:SELECT|VALUES)(.*?)(?:\nON CONFLICT|\nRETURNING|`)").
		FindAllStringSubmatch(source, -1)

	if written := strings.Count(source, "INSERT INTO"); written != len(inserts) {
		t.Fatalf(
			"this package has %d inserts but %d of them are in a shape this guard can read. "+
				"An insert it cannot read is an insert whose columns nobody is checking.",
			written, len(inserts),
		)
	}

	for _, insert := range inserts {
		named := strings.Count(insert[1], ",") + 1

		if bound := strings.Count(insert[2], "$"); named != bound {
			t.Errorf(
				"an insert names %d columns but binds %d values:\n%s\nA column added to the table "+
					"and to the entity but not here is written as its default, so the field "+
					"silently never persists and nothing fails loudly.",
				named, bound, strings.TrimSpace(insert[1]),
			)
		}
	}
}

func TestTheLedgerDatesAnEntryFromTheRowRatherThanFromTheTransaction(t *testing.T) {
	if !strings.Contains(recordLedgerQuery, "created_at_recorded") {
		t.Fatal(
			"the ledger insert never names created_at_recorded, so the column falls back to its " +
				"default of now(). In Postgres now() is the moment the transaction began, and the " +
				"rows this same transaction created were stamped from a Go clock read after that, " +
				"so every entry ends up dated a shade before the row it names. A revert reads those " +
				"two timestamps to tell an edit from the import's own work: it would find every " +
				"single row modified, refuse all of them, and report the import undone with " +
				"nothing removed.",
		)
	}

	if !regexp.MustCompile(`unnest\(\$\d+::timestamptz\[\]\)`).MatchString(recordLedgerQuery) {
		t.Error(
			"created_at_recorded is written from something other than a bound parameter. The one " +
				"timestamp that means anything here is the one the created row itself carries, and " +
				"only the caller holding that row can supply it.",
		)
	}
}

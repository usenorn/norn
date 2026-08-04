package bulkaction

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func source(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatalf("read bulk action repository: %v", err)
	}

	return string(raw)
}

func TestNoBulkQueryCountsIssues(t *testing.T) {
	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)

	if counting.MatchString(source(t)) {
		t.Fatal(
			"a bulk query tallies rows. Bulk must never become a way to ask how many issues match " +
				"something; progress is reported over work already committed to, not over a set " +
				"that was counted first.",
		)
	}
}

func TestNoBulkQueryTakesATableLevelLock(t *testing.T) {
	locking := regexp.MustCompile(`(?i)\bLOCK\s+TABLE\b|\bACCESS\s+EXCLUSIVE\b`)

	if locking.MatchString(source(t)) {
		t.Fatal(
			"a bulk query takes a table-level lock. A bulk run must never block ordinary reads of " +
				"the issue list; row locks inside small transactions are the only permitted form.",
		)
	}
}

func TestProgressIsAdvancedOutsideAnyChunkTransaction(t *testing.T) {
	if !strings.Contains(advanceActionQuery, "processed = processed + ") {
		t.Fatal(
			"progress is not advanced by a relative increment. Assigning an absolute value would " +
				"lose ground whenever two chunks report out of order.",
		)
	}
}

func TestClaimingAndSettlingAreGuardedByTheCurrentStatus(t *testing.T) {
	if !strings.Contains(claimActionQuery, "status = 'queued'") {
		t.Error(
			"claiming does not require the action to still be queued, so a redelivered task would " +
				"start a run that is already in progress and apply every change twice",
		)
	}

	if !strings.Contains(settleActionQuery, "status = 'running'") {
		t.Error("settling does not require the action to be running")
	}
}

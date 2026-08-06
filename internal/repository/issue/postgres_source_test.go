package issue

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestEveryIssueColumnTheInsertSelectsIsAlsoReturnedByIt(t *testing.T) {
	selected := regexp.MustCompile(`\bi\.([a-z_]+)`)

	returning := insertIssueQuery[strings.LastIndex(insertIssueQuery, "RETURNING"):]
	returning = returning[:strings.Index(returning, ")")]

	for _, match := range selected.FindAllStringSubmatch(issueColumns, -1) {
		column := match[1]

		if !regexp.MustCompile(`\b` + column + `\b`).MatchString(returning) {
			t.Errorf(
				"issueColumns reads i.%s but the insert's RETURNING list omits it, so Create "+
					"scans a row that is one column short and every issue creation fails with a "+
					"bare 500. Add %s to the RETURNING list.",
				column, column,
			)
		}
	}
}

func TestAnImportedIssueKeepsItsOwnTwoTimestamps(t *testing.T) {
	written := insertIssueQuery[strings.Index(insertIssueQuery, "SELECT $1"):]
	written = written[:strings.Index(written, "FROM allocated")]

	stamps := regexp.MustCompile(`\$(\d+), \$(\d+)\s*$`).FindStringSubmatch(strings.TrimSpace(written))
	if stamps == nil {
		t.Fatal("could not find the pair the insert binds to created_at and updated_at")
	}

	if stamps[1] == stamps[2] {
		t.Fatalf(
			"created_at and updated_at are both bound to $%s. An issue imported from elsewhere "+
				"arrives with two dates that differ, and collapsing them rewrites its history to "+
				"claim it was never touched after it was filed.",
			stamps[1],
		)
	}
}

func TestProgressCountsOnlyLiveWork(t *testing.T) {
	if !strings.Contains(progressQuery, "i.status = 'active'") {
		t.Fatal(
			"the progress tally counts issues in every lifecycle state. An archived or deleted " +
				"issue still shows in the team's breakdown, so a team that tidies up watches its " +
				"outstanding work go up.",
		)
	}
}

func TestScanIssueTakesExactlyAsManyDestinationsAsIssueColumnsSelects(t *testing.T) {
	selected := 1 + topLevelCommas(issueColumns)

	source, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatalf("read issue repository: %v", err)
	}

	scan := regexp.MustCompile(`(?s)if err := row\.Scan\((.*?)\); err != nil`).FindSubmatch(source)
	if scan == nil {
		t.Fatal("could not find scanIssue's Scan call")
	}

	destinations := strings.Count(string(scan[1]), "&")

	if destinations != selected {
		t.Fatalf(
			"issueColumns selects %d columns but scanIssue passes %d destinations. Every read of "+
				"an issue would fail at runtime with a destination-count mismatch, and no other "+
				"test catches it because none of them reach a database.",
			selected, destinations,
		)
	}
}

func topLevelCommas(columns string) int {
	depth, commas := 0, 0

	for _, r := range columns {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				commas++
			}
		}
	}

	return commas
}

func TestProgressLeavesOutWorkNobodyHasDecidedOn(t *testing.T) {
	if !strings.Contains(progressQuery, "i.triage_state IS DISTINCT FROM 'waiting'") {
		t.Fatal(
			"the progress tally counts issues still waiting in triage. A team watching its " +
				"progress bar move because an integration filed four things overnight is exactly " +
				"the untrustworthy count triage exists to remove.",
		)
	}
}

func TestTheImportDeleteIsFencedToTheRowsItWasHandedInOneWorkspace(t *testing.T) {
	for _, fence := range []string{"workspace_id = $1", "id = ANY($2::uuid[])"} {
		if !strings.Contains(purgeImportedIssuesQuery, fence) {
			t.Errorf(
				"the revert's delete is missing %q. This is the one delete in the repository that "+
					"removes a live issue outright, with no soft delete and no grace period in "+
					"front of it, so the ledger's own ids and the workspace they belong to are the "+
					"whole of what keeps it off somebody else's work.",
				fence,
			)
		}
	}

	if !strings.Contains(purgeImportedIssuesQuery, "status = 'active'") {
		t.Error(
			"the revert's delete will take an issue in any status. An issue somebody archived or " +
				"queued for deletion after the import is no longer the import's to take back out, " +
				"and the caller cannot check that without racing the delete.",
		)
	}
}

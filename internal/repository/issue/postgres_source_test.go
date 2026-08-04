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

package issue

import (
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

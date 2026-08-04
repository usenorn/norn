package issuerelation

import (
	"os"
	"regexp"
	"testing"
)

func TestNoRelationQueryCountsIssues(t *testing.T) {
	source, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatalf("read issue relation repository: %v", err)
	}

	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)

	if counting.MatchString(string(source)) {
		t.Fatal(
			"a relation query tallies rows. The only tally this product may compute is the " +
				"per-category progress breakdown; a relation count is an issue count wearing a " +
				"different hat, and this file is not covered by the guard on the issue repository.",
		)
	}
}

func TestTheRelationListIsFilteredByTheCallersTeamScope(t *testing.T) {
	for name, query := range map[string]string{
		"relationsForIssueQuery": relationsForIssueQuery,
		"relationForPairQuery":   relationForPairQuery,
	} {
		if !regexp.MustCompile(`o\.team_id = ANY`).MatchString(query) {
			t.Errorf(
				"%s does not filter the counterpart by team scope, so a relation to an issue on "+
					"a team the caller cannot see would disclose that the issue exists.",
				name,
			)
		}
	}
}

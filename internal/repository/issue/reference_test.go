package issue_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/repository/issue"
)

func TestIssueNumbersAreAllocatedFromACounterAndNeverFromTheRowsThemselves(t *testing.T) {
	query := issue.InsertIssueQuery

	for _, derived := range []string{"max(", "MAX(", "count(", "COUNT("} {
		if strings.Contains(query, derived) {
			t.Errorf(
				"insertIssueQuery derives the issue number with %s. "+
					"Deleting the highest-numbered issue would then hand its number to the next one, "+
					"and every reference already pasted elsewhere would resolve to the wrong issue. "+
					"Allocate from workspace_issue_numbers instead.",
				derived,
			)
		}
	}

	if !strings.Contains(query, "workspace_issue_numbers") {
		t.Error("insertIssueQuery does not touch workspace_issue_numbers, so numbers are not monotonic")
	}
}

func TestAnIssueCarriesItsOwnFrozenReferenceKeyRatherThanItsTeamsCurrentOne(t *testing.T) {
	if !strings.Contains(issue.IssueColumns, "i.reference_key") {
		t.Fatal(
			"issueColumns does not select i.reference_key. " +
				"Deriving the reference from the joined team key means a team move silently rewrites " +
				"every reference the issue has ever had.",
		)
	}
}

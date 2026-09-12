package issue_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/repository/issue"
)

func TestLookingUpANumberStaysInsideTheWorkspaceAndTheReadersTeams(t *testing.T) {
	query := issue.IssuesByNumberQuery

	if !strings.Contains(query, "i.workspace_id = $1") {
		t.Error(
			"the by-number query does not bind the workspace. A number is unique per team, not " +
				"globally, so an unbound query reaches into every workspace on the instance.",
		)
	}

	if !strings.Contains(query, "i.team_id = ANY($4::uuid[])") {
		t.Error(
			"the by-number query does not apply the team scope. Every other read carries it, and " +
				"search would otherwise name issues on teams the reader cannot open.",
		)
	}

	if !strings.Contains(query, "LIMIT $5") {
		t.Error("the by-number query is unbounded, so one typed digit can return a whole workspace")
	}
}

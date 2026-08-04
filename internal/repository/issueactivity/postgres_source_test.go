package issueactivity

import (
	"regexp"
	"strings"
	"testing"
)

func TestTheActivityInsertWritesEveryColumnItNames(t *testing.T) {
	columns := regexp.MustCompile(`(?s)INSERT INTO workspace_issue_activity \((.*?)\)`).
		FindStringSubmatch(recordActivityQuery)
	if columns == nil {
		t.Fatal("could not find the activity insert's column list")
	}

	named := strings.Count(columns[1], ",") + 1

	values := regexp.MustCompile(`VALUES \((.*?)\)`).FindStringSubmatch(recordActivityQuery)
	if values == nil {
		t.Fatal("could not find the activity insert's VALUES list")
	}

	placeholders := strings.Count(values[1], "$")

	if named != placeholders {
		t.Fatalf(
			"the insert names %d columns but binds %d values. A column added to the table and to "+
				"the entity but not here is written as its default, so the field silently never "+
				"persists and nothing fails loudly.",
			named, placeholders,
		)
	}
}

func TestWhatTheActivityInsertWritesTheActivityReadReturns(t *testing.T) {
	columns := regexp.MustCompile(`(?s)INSERT INTO workspace_issue_activity \((.*?)\)`).
		FindStringSubmatch(recordActivityQuery)
	if columns == nil {
		t.Fatal("could not find the activity insert's column list")
	}

	skipped := map[string]bool{"actor_account_id": true, "from_state_id": true, "to_state_id": true}

	for _, raw := range strings.Split(columns[1], ",") {
		column := strings.TrimSpace(raw)

		if column == "" || skipped[column] {
			continue
		}

		if !strings.Contains(activityPageQuery, column) {
			t.Errorf(
				"the insert writes %s but the page query never selects it, so the value is stored "+
					"and then thrown away on every read",
				column,
			)
		}
	}
}

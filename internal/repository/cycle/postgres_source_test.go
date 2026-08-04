package cycle

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func repositorySources(t *testing.T) map[string]string {
	t.Helper()

	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list cycle repository sources: %v", err)
	}

	sources := map[string]string{}

	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		sources[name] = string(source)
	}

	if len(sources) == 0 {
		t.Fatal("found no cycle repository sources to guard")
	}

	return sources
}

func TestNoCycleQueryCountsIssues(t *testing.T) {
	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)

	for name, source := range repositorySources(t) {
		if counting.MatchString(source) {
			t.Errorf(
				"%s tallies rows. A cycle's only tally is the per-category progress breakdown, "+
					"which is computed by progressQuery in the issue repository; counting here "+
					"would put an issue count on a surface no guard covers.",
				name,
			)
		}
	}
}

func TestEveryVisibleCycleQueryIsFilteredByTeamScope(t *testing.T) {
	scoped := regexp.MustCompile(`c\.team_id = ANY`)

	for name, query := range map[string]string{
		"visibleCycleQuery":  visibleCycleQuery,
		"cycleByNumberQuery": cycleByNumberQuery,
		"visibleCyclesQuery": visibleCyclesQuery,
	} {
		if !scoped.MatchString(query) {
			t.Errorf(
				"%s does not filter by team scope, so a cycle belonging to a team the caller "+
					"cannot see would disclose that the team plans work.",
				name,
			)
		}
	}
}

func TestClosingACycleIsConditionalOnItBeingOpen(t *testing.T) {
	if !strings.Contains(closeCycleQuery, "closed_at IS NULL") {
		t.Fatal(
			"closeCycleQuery does not require the cycle to be open, so a second close would " +
				"overwrite the rollover decision recorded by the first.",
		)
	}
}

package project

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
		t.Fatalf("list project repository sources: %v", err)
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
		t.Fatal("found no project repository sources to guard")
	}

	return sources
}

func TestNoProjectQueryCountsIssues(t *testing.T) {
	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)

	for name, source := range repositorySources(t) {
		if counting.MatchString(source) {
			t.Errorf(
				"%s tallies rows. A project's only tally is the per-category progress breakdown, "+
					"which progressQuery computes inside the caller's team scope; counting here "+
					"would report work the caller is not allowed to see.",
				name,
			)
		}
	}
}

func TestConcealedWorkAsksWhetherAnyExistsRatherThanHowMuch(t *testing.T) {
	if !strings.Contains(concealedWorkQuery, "SELECT EXISTS") {
		t.Fatal(
			"concealedWorkQuery no longer asks for a bare existence check. Reporting how many " +
				"issues sit on teams the caller cannot see would disclose, by subtraction, the " +
				"per-category breakdown of a private team's work.",
		)
	}
}

func TestArchivingIsConditionalOnTheProjectBeingOpen(t *testing.T) {
	if !strings.Contains(archiveProjectQuery, "archived_at IS NULL") {
		t.Fatal("archiveProjectQuery does not require the project to be open, so archiving twice would restamp it")
	}

	if !strings.Contains(unarchiveProjectQuery, "archived_at IS NOT NULL") {
		t.Fatal("unarchiveProjectQuery does not require the project to be archived")
	}
}

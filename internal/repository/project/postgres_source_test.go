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

func TestAnImportedProjectKeepsItsOwnTwoTimestamps(t *testing.T) {
	if !strings.Contains(insertProjectQuery, "created_at") || !strings.Contains(insertProjectQuery, "updated_at") {
		t.Fatal(
			"insertProjectQuery leaves created_at or updated_at to the column default, so a " +
				"project imported from elsewhere is dated the moment the import ran",
		)
	}

	values := regexp.MustCompile(`(?s)VALUES \((.*?)\)`).FindStringSubmatch(insertProjectQuery)
	if values == nil {
		t.Fatal("could not read insertProjectQuery's bound values")
	}

	bound := strings.Split(values[1], ",")
	if len(bound) < 2 {
		t.Fatal("insertProjectQuery binds fewer values than it names columns")
	}

	created := strings.TrimSpace(bound[len(bound)-2])
	updated := strings.TrimSpace(bound[len(bound)-1])

	if created == updated {
		t.Fatalf(
			"created_at and updated_at are both bound to %s. A project imported with a later "+
				"revision date would report that nobody has touched it since it was created.",
			created,
		)
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

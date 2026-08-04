package savedview

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
		t.Fatalf("list saved view repository sources: %v", err)
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
		t.Fatal("found no saved view repository sources to guard")
	}

	return sources
}

func TestNoSavedViewQueryCountsAnything(t *testing.T) {
	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)

	for name, source := range repositorySources(t) {
		if counting.MatchString(source) {
			t.Errorf(
				"%s counts rows. A saved view must never report how many issues it matches, nor "+
					"how many people can see it. The first is an issue count on a surface no scope "+
					"guard covers; the second is a membership count wearing a different hat.",
				name,
			)
		}
	}
}

func TestTheOrderingIntegerNeverLeavesTheQuery(t *testing.T) {
	sources := repositorySources(t)
	source := sources["postgres.go"]

	if !strings.Contains(source, "ORDER BY p.position NULLS LAST") {
		t.Fatal("the listing no longer orders by the caller's own placement")
	}

	for _, projection := range []string{"p.position,", "p.position\n", "v.position"} {
		if strings.Contains(source, "SELECT"+projection) || strings.Contains(source, projection+"\n       ") {
			t.Errorf(
				"the placement integer appears in a projection (%q). Order is expressed by the "+
					"order of the returned rows; the moment a position reaches a caller it can be "+
					"cached, go stale, and disagree with the list it came from.",
				projection,
			)
		}
	}
}

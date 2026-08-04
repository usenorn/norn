package issue_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func issueRepositorySource(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}

	source, err := os.ReadFile(filepath.Join(dir, "postgres.go"))
	if err != nil {
		t.Fatalf("read issue repository: %v", err)
	}

	return string(source)
}

func TestEveryWriteToAnIssueStampsTheFieldItChanged(t *testing.T) {
	source := issueRepositorySource(t)

	statement := regexp.MustCompile(`(?s)UPDATE workspace_issues\s+SET.*?`)

	for _, match := range statement.FindAllStringIndex(source, -1) {
		tail := source[match[0]:]

		end := strings.Index(tail, "`")
		if end < 0 {
			end = len(tail)
		}

		if !strings.Contains(tail[:end], "field_versions") {
			t.Errorf(
				"an UPDATE of workspace_issues does not stamp field_versions:\n\n%s\n\n"+
					"Every write must record which field it changed, or a concurrent editor's "+
					"conflict check cannot see it and one party's change is silently discarded.",
				strings.TrimSpace(tail[:end]),
			)
		}
	}
}

func TestNoIssueQueryCountsIssuesOutsideTheProgressTally(t *testing.T) {
	source := issueRepositorySource(t)

	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)

	for block := range strings.SplitSeq(source, "const ") {
		name, _, found := strings.Cut(block, " =")
		if !found {
			continue
		}

		if strings.TrimSpace(name) == "progressQuery" {
			continue
		}

		if counting.MatchString(block) {
			t.Errorf(
				"%s counts rows. The only tally this product may compute is the per-category "+
					"progress breakdown; anything else becomes an issue count that could be "+
					"metered or capped.",
				strings.TrimSpace(name),
			)
		}
	}
}

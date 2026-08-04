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

	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list issue repository sources: %v", err)
	}

	var source strings.Builder

	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		contents, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		source.Write(contents)
	}

	if source.Len() == 0 {
		t.Fatal("found no issue repository sources to guard")
	}

	return source.String()
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

func TestEveryTallyIsTakenInsideTheCallersScope(t *testing.T) {
	source := issueRepositorySource(t)

	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)
	scoped := regexp.MustCompile(`i\.team_id = ANY\(`)

	for _, line := range strings.Split(source, "\n") {
		if !counting.MatchString(line) {
			continue
		}

		if strings.Contains(line, "// unscoped:") {
			t.Errorf("a tally is marked unscoped: %s", strings.TrimSpace(line))
		}
	}

	if !counting.MatchString(source) {
		return
	}

	for _, statement := range countingStatements(source) {
		if scoped.MatchString(statement) {
			continue
		}

		t.Errorf(
			"a query counts rows without the caller's team scope:\n%s\n"+
				"Counting is allowed here, but only inside the scope the caller can already "+
				"enumerate by paging. A tally that reaches past it discloses, by subtraction, "+
				"the contents of a team the guard conceals.",
			strings.TrimSpace(statement),
		)
	}
}

// countingStatements returns the handwritten SQL literals that tally rows. SQL assembled at
// runtime is not visible here and is covered by TestEveryTallyIsBuiltInsideTheCallersScope,
// which inspects what the builder actually emits.
func countingStatements(source string) []string {
	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)

	statements := []string{}
	literals := strings.Split(source, "`")

	for i := 1; i < len(literals); i += 2 {
		if counting.MatchString(literals[i]) {
			statements = append(statements, literals[i])
		}
	}

	return statements
}

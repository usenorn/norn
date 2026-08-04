package issuefilterreference

import (
	"regexp"
	"strings"
	"testing"
)

func TestNoResolutionQueryCountsAnything(t *testing.T) {
	if regexp.MustCompile(`(?i)\b(count|sum)\s*\(`).MatchString(resolveReferencesQuery) {
		t.Error(
			"the resolution query counts. It answers what a filter points at, one row per id; " +
				"a tally here would report the size of something the caller may not be able to list.",
		)
	}
}

func TestTheResolutionQueryNeverSelectsANameItMayNotShow(t *testing.T) {
	guarded := strings.Count(resolveReferencesQuery, "ELSE '' END")

	if guarded < 4 {
		t.Fatalf(
			"only %d of the resolution branches withhold their name behind a CASE. A name the "+
				"caller may not see must never leave Postgres — fetching it and dropping it in Go "+
				"is one refactor away from being forgotten.",
			guarded,
		)
	}
}

func TestEveryResolutionBranchIsConfinedToOneWorkspace(t *testing.T) {
	branches := strings.Count(resolveReferencesQuery, "FROM workspace_")
	confined := strings.Count(resolveReferencesQuery, ".workspace_id = $1")

	if confined < branches-1 {
		t.Fatalf(
			"%d of %d branches are workspace-confined. An id pasted from another workspace must "+
				"read as missing, never as restricted: restricted would confirm that the id names "+
				"something real somewhere.",
			confined, branches,
		)
	}
}

func TestEveryNamingFieldHasAKind(t *testing.T) {
	for field, kind := range referenceKinds {
		if !field.Names() {
			t.Errorf("%q is mapped to kind %q but the domain says it names nothing", field, kind)
		}
	}
}

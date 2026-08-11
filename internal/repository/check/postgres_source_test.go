package check

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/repository"
)

func packageSource(t *testing.T) string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading this package: %v", err)
	}

	var combined strings.Builder

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		combined.Write(body)
	}

	if combined.Len() == 0 {
		t.Fatal("this guard found no source, so it is protecting nothing")
	}

	return combined.String()
}

func TestNothingRewritesAPieceOfEvidence(t *testing.T) {
	rewriting := regexp.MustCompile(`(?is)update\s+workspace_check_evidence`)

	if rewriting.MatchString(packageSource(t)) {
		t.Error(
			"this package rewrites stored evidence. Evidence is append-only: a newer observation " +
				"is a new record, never an edit to an old one, because a check's state is read " +
				"from what was filed and when.",
		)
	}
}

func TestNothingRemovesAPieceOfEvidenceOnItsOwn(t *testing.T) {
	deleting := regexp.MustCompile(`(?is)delete\s+from\s+workspace_check_evidence`)

	if deleting.MatchString(packageSource(t)) {
		t.Error(
			"this package deletes evidence directly. Evidence goes only when the check it was " +
				"filed against goes, which the foreign key already does.",
		)
	}
}

func TestTheEvidenceSurfaceOffersNoWayToChangeWhatWasObserved(t *testing.T) {
	forbidden := []string{"update", "delete", "remove", "edit", "redact", "set"}

	surface := reflect.TypeOf((*repository.CheckEvidence)(nil)).Elem()

	for method := range surface.NumMethod() {
		name := strings.ToLower(surface.Method(method).Name)

		for _, verb := range forbidden {
			if strings.HasPrefix(name, verb) {
				t.Errorf(
					"repository.CheckEvidence offers %q. Nothing may change an observation after "+
						"it was filed; submit a newer one instead.",
					surface.Method(method).Name,
				)
			}
		}
	}
}

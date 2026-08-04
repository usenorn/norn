package entity_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAReferenceReadsTheFrozenKeyNotTheTeamTheIssueCurrentlyLivesIn(t *testing.T) {
	moved := entity.Issue{
		ID:           uuid.New(),
		TeamKey:      "PLT",
		ReferenceKey: "MOB",
		Number:       14,
	}

	if got := moved.Reference(); got != "MOB-14" {
		t.Fatalf(
			"Reference() = %q, want MOB-14 — an issue moved to another team keeps the reference "+
				"every existing link already quotes",
			got,
		)
	}
}

func TestNothingOnTheIssueSurfaceCanRewriteAReference(t *testing.T) {
	frozen := []string{"referencekey", "number"}

	issue := reflect.TypeOf(entity.Issue{})

	for _, name := range frozen {
		field, ok := issue.FieldByNameFunc(func(candidate string) bool {
			return strings.EqualFold(candidate, name)
		})

		if !ok {
			t.Errorf("entity.Issue has no %q field, so a reference cannot be frozen at creation", name)

			continue
		}

		if field.Type.Kind() == reflect.Pointer {
			t.Errorf(
				"entity.Issue.%s is a pointer, which invites a nil-means-recompute path. "+
					"A frozen reference must be a plain value written once at creation.",
				field.Name,
			)
		}
	}
}

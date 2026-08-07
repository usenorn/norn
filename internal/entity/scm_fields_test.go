package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestALabelIsMatchedByNameWhateverItsCase(t *testing.T) {
	bug := entity.Label{ID: uuid.New(), Name: "Bug"}
	urgent := entity.Label{ID: uuid.New(), Name: "urgent"}

	matched, unmapped := entity.MapLabels(
		[]string{"bug", "URGENT", "needs-triage", " bug "},
		[]entity.Label{bug, urgent},
	)

	if len(matched) != 2 {
		t.Fatalf("matched %d labels, want 2", len(matched))
	}

	if matched[0].ID != bug.ID || matched[1].ID != urgent.ID {
		t.Fatalf("matched the wrong labels: %v", matched)
	}

	if len(unmapped) != 1 || unmapped[0] != "needs-triage" {
		t.Fatalf(
			"unmapped = %v; a platform label nobody here has must be reported rather than "+
				"silently dropped or invented in somebody's workspace",
			unmapped,
		)
	}
}

func TestAWorkspaceWithNoMatchingLabelsGetsNoneRatherThanNew(t *testing.T) {
	matched, unmapped := entity.MapLabels([]string{"bug", "urgent"}, nil)

	if len(matched) != 0 {
		t.Fatalf("matched %d labels against an empty workspace", len(matched))
	}

	if len(unmapped) != 2 {
		t.Fatalf("unmapped = %v, want both reported", unmapped)
	}
}

func TestLabelNamesDropsTheBlanks(t *testing.T) {
	names := entity.LabelNames([]entity.Label{
		{Name: "bug"},
		{Name: "   "},
		{Name: " urgent "},
	})

	if len(names) != 2 || names[0] != "bug" || names[1] != "urgent" {
		t.Fatalf("LabelNames = %v, want [bug urgent]", names)
	}
}

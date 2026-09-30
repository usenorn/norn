package entity_test

import (
	"errors"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAPreviewOutcomeIsOneOfReadyFailedOrUnsupported(t *testing.T) {
	valid := map[entity.SnapshotPreviewState]bool{
		entity.SnapshotPreviewReady:       true,
		entity.SnapshotPreviewFailed:      true,
		entity.SnapshotPreviewUnsupported: true,
		"":                                false,
		"open":                            false,
		"skipped":                         false,
	}

	for state, want := range valid {
		if state.Valid() != want {
			t.Errorf("%q reports Valid() as %t", state, !want)
		}
	}
}

func TestAPreviewThatNamesNoServiceIsRefused(t *testing.T) {
	err := entity.ValidateSnapshotPreview("previews[0]", entity.SnapshotPreview{
		Name:  "Application",
		State: entity.SnapshotPreviewFailed,
	})

	var invalid entity.ValidationError

	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "previews[0].service" {
		t.Fatalf("a preview with no service answered %v", err)
	}
}

func TestAPreviewOnAPortNoMachineHasIsRefused(t *testing.T) {
	err := entity.ValidateSnapshotPreview("previews[0]", entity.SnapshotPreview{
		Name:    "Application",
		Service: "web",
		State:   entity.SnapshotPreviewReady,
		Port:    70000,
	})

	var invalid entity.ValidationError

	if !errors.As(err, &invalid) || invalid.Fields[0].Code != entity.ValidationCodeOutOfRange {
		t.Fatalf("port 70000 answered %v", err)
	}
}

func TestASnapshotsHeadsAreItsRepositoriesHeads(t *testing.T) {
	heads := entity.ExecutionSnapshot{Repositories: []entity.SnapshotRepository{
		{Repository: "backend", HeadSHA: "b1"},
		{Repository: "frontend", HeadSHA: "f1"},
	}}.Heads()

	if !heads.Matches(entity.ReviewHeads{"backend": "b1", "frontend": "f1"}) {
		t.Fatalf("the heads read %v", heads)
	}
}

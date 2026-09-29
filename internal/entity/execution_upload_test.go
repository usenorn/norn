package entity_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnUploadsKeyNamesTheWorkspaceBeforeTheRun(t *testing.T) {
	workspaceID := uuid.New()
	executionID := entity.NewExecutionID("01ABC")
	artifactID := uuid.New()

	prefix := entity.ExecutionBlobPrefix(workspaceID)

	for name, key := range map[string]string{
		"an artifact": entity.ExecutionArtifactKey(workspaceID, executionID, artifactID),
	} {
		if !strings.HasPrefix(key, prefix+"/") {
			t.Errorf(
				"%s is stored at %q, which the workspace purge sweeping %q would leave behind",
				name, key, prefix,
			)
		}

		if !entity.ValidBlobKey(key) {
			t.Errorf("%s is stored at %q, which is not a key the store accepts", name, key)
		}
	}
}

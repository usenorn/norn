package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func TestAChangeSetReachesTheBrowserCarryingWhatChanged(t *testing.T) {
	changeset := entity.ExecutionChangeSet{
		ExecutionID: "exec-01ABC",
		Result: entity.ExecutionResult{
			ExecutionID: "exec-01ABC",
			Summary:     "added a median helper to the ledger",
			ReportedAt:  time.Now().UTC(),
		},
		Changes: []entity.ExecutionChange{{
			ID:           uuid.New(),
			ExecutionID:  "exec-01ABC",
			Repository:   "northwind",
			Branch:       "norn/NORN-61/northwind",
			Commits:      3,
			Additions:    412,
			Deletions:    77,
			FilesChanged: 9,
			ReportedAt:   time.Now().UTC(),
		}},
	}

	payload, err := json.Marshal(changeset)
	if err != nil {
		t.Fatalf("write a changeset: %v", err)
	}

	translated := translate(entity.Event{
		Kind:    entity.EventExecutionChangeSet,
		Payload: payload,
	})

	var carried api.ExecutionChangeSet

	if err := json.Unmarshal(translated, &carried); err != nil {
		t.Fatalf("read what the browser is sent: %v", err)
	}

	if carried.ExecutionId != "exec-01ABC" {
		t.Fatalf(
			"the event named run %q, so a screen cannot tell whether the change set is the one "+
				"it is showing",
			carried.ExecutionId,
		)
	}

	if len(carried.Repositories) != 1 || carried.Repositories[0].Repository != "northwind" {
		t.Fatalf(
			"what changed did not survive translation: %+v. A change set event that arrives "+
				"empty is one every watching screen has to refetch behind",
			carried.Repositories,
		)
	}
}

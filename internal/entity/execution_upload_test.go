package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyTheStreamsAndTelemetryModesTheSchemaAllowsAreValid(t *testing.T) {
	for _, stream := range entity.ExecutionStreams() {
		if !stream.Valid() {
			t.Errorf("%q is offered as a stream but does not pass its own check", stream)
		}
	}

	if entity.ExecutionStream("stderr").Valid() {
		t.Error("a stream the table's check constraint would refuse passed validation")
	}

	for _, mode := range entity.TelemetryModes() {
		if !mode.Valid() {
			t.Errorf("%q is offered as a telemetry mode but does not pass its own check", mode)
		}
	}

	if entity.TelemetryMode("none").Valid() {
		t.Error("a telemetry mode the table's check constraint would refuse passed validation")
	}
}

func TestMinimalTelemetryDropsTheTranscriptAndKeepsTheOutput(t *testing.T) {
	if entity.TelemetryMinimal.Keeps(entity.ExecutionStreamTranscript) {
		t.Error("a workspace keeping summaries only would still be sent a full transcript")
	}

	if !entity.TelemetryMinimal.Keeps(entity.ExecutionStreamLogs) {
		t.Error("a workspace keeping summaries only lost the command output as well")
	}

	for _, stream := range entity.ExecutionStreams() {
		if !entity.TelemetryFull.Keeps(stream) {
			t.Errorf("full telemetry turned down %q", stream)
		}
	}
}

func TestAWorkspaceWithNoPolicyRowFallsBackToWhatTheInstanceKeeps(t *testing.T) {
	settled := entity.WorkspaceExecutionPolicy{WorkspaceID: uuid.New()}.
		Normalised(90 * 24 * time.Hour)

	if settled.Telemetry != entity.TelemetryFull || settled.UploadRetentionDays != 90 {
		t.Fatalf("an unconfigured workspace settled as %+v", settled)
	}

	kept := entity.WorkspaceExecutionPolicy{
		WorkspaceID: uuid.New(), Telemetry: entity.TelemetryMinimal, UploadRetentionDays: 7,
	}.Normalised(90 * 24 * time.Hour)

	if kept.Telemetry != entity.TelemetryMinimal || kept.UploadRetentionDays != 7 {
		t.Fatalf("a configured workspace was overwritten with the default: %+v", kept)
	}
}

func TestARetentionWindowShorterThanADayStillKeepsADay(t *testing.T) {
	if days := entity.ExecutionRetentionDays(time.Hour); days != 1 {
		t.Fatalf("a window of an hour came out as %d days, which would sweep on write", days)
	}
}

func TestAnUploadsKeyNamesTheWorkspaceBeforeTheRun(t *testing.T) {
	workspaceID := uuid.New()
	executionID := entity.NewExecutionID("01ABC")
	artifactID := uuid.New()

	prefix := entity.ExecutionBlobPrefix(workspaceID)

	for name, key := range map[string]string{
		"a chunk":     entity.ExecutionChunkKey(workspaceID, executionID, entity.ExecutionStreamLogs, "abc123"),
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

func TestAskingForMoreChunksThanAPageHoldsIsClamped(t *testing.T) {
	page := entity.ExecutionChunkPage{Limit: 5000, After: -3}.Normalized()

	if page.Limit != entity.ExecutionChunkPageMax || page.After != 0 {
		t.Fatalf("the page settled as %+v", page)
	}

	if empty := (entity.ExecutionChunkPage{}).Normalized(); empty.Limit != entity.ExecutionChunkPageDefault {
		t.Fatalf("a page nobody sized settled as %+v", empty)
	}
}

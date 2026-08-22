package executionupload_test

import (
	"context"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestPruningTakesAnAgedTranscriptAndTheObjectBehindIt(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	if _, err := h.service.AppendTranscript(ctx, h.execution.ID, service.TranscriptBatch{
		Sequence: 1,
		Entries: []entity.ExecutionTranscriptEntry{
			{At: time.Now().UTC(), Type: "message", Payload: map[string]any{"role": "assistant"}},
		},
	}); err != nil {
		t.Fatalf("send a transcript chunk: %v", err)
	}

	key := h.chunks[0].ObjectKey

	if !h.stored(t, key) {
		t.Fatal("the transcript was never written to storage")
	}

	h.chunks[0].ReceivedAt = time.Now().UTC().AddDate(0, 0, -120)

	if err := h.service.Prune(ctx); err != nil {
		t.Fatalf("prune what has aged out: %v", err)
	}

	if len(h.chunks) != 0 {
		t.Fatalf("%d aged chunks are still on record", len(h.chunks))
	}

	if h.stored(t, key) {
		t.Fatal("the row went and the stored object stayed behind")
	}
}

func TestPruningLeavesWhatIsStillInsideTheWindow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	if _, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(1, "recent"),
	}); err != nil {
		t.Fatalf("send a batch of output: %v", err)
	}

	if err := h.service.Prune(ctx); err != nil {
		t.Fatalf("prune what has aged out: %v", err)
	}

	if len(h.chunks) != 1 {
		t.Fatalf("a batch inside the window was swept; %d remain", len(h.chunks))
	}
}

func TestAWorkspaceThatKeepsThingsLongerIsObeyedOverTheInstanceDefault(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.policy = entity.WorkspaceExecutionPolicy{
		WorkspaceID: h.workspaceID, Telemetry: entity.TelemetryFull, UploadRetentionDays: 365,
	}

	if _, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(1, "kept longer"),
	}); err != nil {
		t.Fatalf("send a batch of output: %v", err)
	}

	h.chunks[0].ReceivedAt = time.Now().UTC().AddDate(0, 0, -120)

	if err := h.service.Prune(ctx); err != nil {
		t.Fatalf("prune what has aged out: %v", err)
	}

	if len(h.chunks) != 1 {
		t.Fatal("a workspace asking for a year of history lost a batch at ninety days")
	}
}

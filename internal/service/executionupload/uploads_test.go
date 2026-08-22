package executionupload_test

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestABatchOfOutputReadsBackAsTheMachineSentIt(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	sent := h.logs(3, "compiling")

	receipt, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: sent,
	})
	if err != nil {
		t.Fatalf("send a batch of output: %v", err)
	}

	if receipt.Duplicate || receipt.Chunk.Entries != 3 || receipt.Chunk.Digest == "" {
		t.Fatalf("the batch came back as %+v", receipt)
	}

	read, err := h.service.Logs(ctx, h.workspaceID, h.execution.ID, entity.ExecutionChunkPage{})
	if err != nil {
		t.Fatalf("read the output back: %v", err)
	}

	if len(read) != 1 || len(read[0].Entries) != 3 {
		t.Fatalf("the output came back as %+v", read)
	}

	for index, entry := range read[0].Entries {
		if entry.Text != sent[index].Text || !entry.At.Equal(sent[index].At) ||
			entry.Source != sent[index].Source {
			t.Fatalf("line %d came back as %+v, want %+v", index, entry, sent[index])
		}
	}
}

func TestTheSameBatchSentTwiceIsStoredOnceAndSaysSo(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	batch := service.LogBatch{Sequence: 1, Entries: h.logs(2, "linking")}

	first, err := h.service.AppendLogs(ctx, h.execution.ID, batch)
	if err != nil {
		t.Fatalf("send a batch of output: %v", err)
	}

	again, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
		Sequence: batch.Sequence, Entries: h.logs(2, "linking"),
	})
	if err != nil {
		t.Fatalf("send the same batch again: %v", err)
	}

	if !again.Duplicate {
		t.Fatal("a batch replayed after a reconnect was taken as new work")
	}

	if again.Chunk.Digest != first.Chunk.Digest || again.Chunk.ID != first.Chunk.ID {
		t.Fatalf("the replay came back as %+v, want the receipt of %+v", again.Chunk, first.Chunk)
	}

	if len(h.chunks) != 1 {
		t.Fatalf("the run holds %d batches after sending one twice", len(h.chunks))
	}
}

func TestADifferentBatchAtATakenPositionIsRefusedByName(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	if _, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(1, "first"),
	}); err != nil {
		t.Fatalf("send a batch of output: %v", err)
	}

	_, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(1, "second"),
	})
	if !errors.Is(err, entity.ErrExecutionChunkConflict) {
		t.Fatalf("a second batch at position 1 answered %v", err)
	}
}

func TestAMachineThatDoesNotHoldTheRunIsToldThereIsNoSuchRun(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.AppendLogs(context.Background(), entity.NewExecutionID("01OTHER"), service.LogBatch{
		Sequence: 1, Entries: h.logs(1, "not mine"),
	})
	if !errors.Is(err, entity.ErrExecutionNotFound) {
		t.Fatalf(
			"uploading against a run this machine does not hold answered %v; anything but "+
				"not-found lets a machine probe for runs it was never given",
			err,
		)
	}

	if len(h.chunks) != 0 {
		t.Fatalf("%d batches were stored for a run the machine does not hold", len(h.chunks))
	}
}

func TestAFinishedRunTakesNoMoreUploads(t *testing.T) {
	h := newHarness(t)
	h.execution.State = entity.ExecutionCompleted

	_, err := h.service.AppendLogs(context.Background(), h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(1, "too late"),
	})
	if !errors.Is(err, entity.ErrExecutionFinished) {
		t.Fatalf("uploading against a finished run answered %v", err)
	}
}

func TestABatchLargerThanTheInstanceAcceptsIsRefusedRatherThanCutShort(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.AppendLogs(context.Background(), h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(20, strings.Repeat("x", 512)),
	})
	if !errors.Is(err, entity.ErrExecutionUploadTooLarge) {
		t.Fatalf("an oversized batch answered %v", err)
	}

	if len(h.chunks) != 0 {
		t.Fatalf("%d batches were stored from an upload that was refused", len(h.chunks))
	}
}

func TestARunThatHasUploadedItsAllowanceIsRefusedByName(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	for sequence := int64(1); sequence <= 200; sequence++ {
		entries := make([]entity.ExecutionLogEntry, 0, 8)

		for range 8 {
			entries = append(entries, entity.ExecutionLogEntry{
				At: time.Now().UTC(), Stream: "stdout", Text: uuid.NewString() + uuid.NewString(),
			})
		}

		_, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
			Sequence: sequence, Entries: entries,
		})
		if err == nil {
			continue
		}

		if errors.Is(err, entity.ErrExecutionUploadExhausted) {
			return
		}

		t.Fatalf("batch %d answered %v", sequence, err)
	}

	t.Fatal("a run uploaded past its allowance and nothing turned it down")
}

func TestAWorkspaceKeepingSummariesOnlyRefusesATranscriptAndStillTakesOutput(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.policy = entity.WorkspaceExecutionPolicy{
		WorkspaceID: h.workspaceID, Telemetry: entity.TelemetryMinimal, UploadRetentionDays: 90,
	}

	_, err := h.service.AppendTranscript(ctx, h.execution.ID, service.TranscriptBatch{
		Sequence: 1,
		Entries: []entity.ExecutionTranscriptEntry{
			{At: time.Now().UTC(), Type: "message", Payload: map[string]any{"role": "assistant"}},
		},
	})
	if !errors.Is(err, entity.ErrExecutionTelemetryMinimal) {
		t.Fatalf("a transcript sent to a minimal workspace answered %v", err)
	}

	if _, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(1, "still building"),
	}); err != nil {
		t.Fatalf("a minimal workspace turned down its own command output: %v", err)
	}
}

func TestATranscriptChunkReadsBackWithWhatTheDriverPutInIt(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	if _, err := h.service.AppendTranscript(ctx, h.execution.ID, service.TranscriptBatch{
		Sequence: 4,
		Entries: []entity.ExecutionTranscriptEntry{{
			At:      time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC),
			Type:    "tool_call",
			Payload: map[string]any{"name": "Edit", "path": "internal/entity/run.go"},
		}},
	}); err != nil {
		t.Fatalf("send a transcript chunk: %v", err)
	}

	read, err := h.service.Transcript(ctx, h.workspaceID, h.execution.ID, entity.ExecutionChunkPage{})
	if err != nil {
		t.Fatalf("read the transcript back: %v", err)
	}

	if len(read) != 1 || len(read[0].Entries) != 1 {
		t.Fatalf("the transcript came back as %+v", read)
	}

	entry := read[0].Entries[0]
	if entry.Type != "tool_call" || entry.Payload["name"] != "Edit" {
		t.Fatalf("the entry came back as %+v", entry)
	}
}

func TestWhatEachStreamHasReachedIsWhatAMachineResumesFrom(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	for sequence := int64(1); sequence <= 3; sequence++ {
		if _, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
			Sequence: sequence, Entries: h.logs(2, "step "+strconv.FormatInt(sequence, 10)),
		}); err != nil {
			t.Fatalf("send batch %d: %v", sequence, err)
		}
	}

	cursors, err := h.service.Cursors(ctx, h.execution.ID)
	if err != nil {
		t.Fatalf("read the stream cursors: %v", err)
	}

	if len(cursors) != 1 || cursors[0].LastSequence != 3 || cursors[0].Chunks != 3 {
		t.Fatalf("the cursors came back as %+v", cursors)
	}
}

func TestAFilePublishedTwiceKeepsTheIdItWasFirstGiven(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	first, err := h.service.SaveArtifact(ctx, h.execution.ID, service.ArtifactUpload{
		Name:        "backend.diff",
		ContentType: "text/plain",
		Body:        bytes.NewReader([]byte("--- a/main.go\n+++ b/main.go\n")),
	})
	if err != nil {
		t.Fatalf("publish an artifact: %v", err)
	}

	again, err := h.service.SaveArtifact(ctx, h.execution.ID, service.ArtifactUpload{
		Name:        "backend.diff",
		ContentType: "text/plain",
		Body:        bytes.NewReader([]byte("--- a/main.go\n+++ b/main.go\n")),
	})
	if err != nil {
		t.Fatalf("publish the same artifact again: %v", err)
	}

	if !again.Duplicate || again.Artifact.ID != first.Artifact.ID {
		t.Fatalf("the second publish came back as %+v, want the id of %+v", again, first.Artifact)
	}

	if len(h.artifacts) != 1 {
		t.Fatalf("the run holds %d artifacts after publishing one twice", len(h.artifacts))
	}

	if h.stored(t, entity.ExecutionArtifactKey(h.workspaceID, h.execution.ID, again.Artifact.ID)) &&
		again.Artifact.ID == first.Artifact.ID {
		return
	}

	t.Fatal("the artifact that was kept is not the one still in storage")
}

func TestAnArtifactIsListedAndCanBeFetchedBack(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	saved, err := h.service.SaveArtifact(ctx, h.execution.ID, service.ArtifactUpload{
		Name:        "screenshot.png",
		ContentType: "image/png",
		Body:        bytes.NewReader([]byte("\x89PNG\r\n\x1a\n")),
	})
	if err != nil {
		t.Fatalf("publish an artifact: %v", err)
	}

	listed, err := h.service.Artifacts(ctx, h.workspaceID, h.execution.ID)
	if err != nil || len(listed) != 1 || listed[0].ID != saved.Artifact.ID {
		t.Fatalf("the artifacts came back as %+v (%v)", listed, err)
	}
}

func TestAnArtifactLargerThanTheInstanceAcceptsIsRefusedAndLeavesNothingBehind(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.SaveArtifact(context.Background(), h.execution.ID, service.ArtifactUpload{
		Name:        "core.dump",
		ContentType: "application/octet-stream",
		Body:        bytes.NewReader(make([]byte, uploadLimit+1)),
	})
	if !errors.Is(err, entity.ErrExecutionUploadTooLarge) {
		t.Fatalf("an oversized artifact answered %v", err)
	}

	if len(h.artifacts) != 0 {
		t.Fatalf("%d artifacts were recorded from an upload that was refused", len(h.artifacts))
	}
}

func TestAnEmptyArtifactIsRefusedRatherThanRecordedAsNothing(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.SaveArtifact(context.Background(), h.execution.ID, service.ArtifactUpload{
		Name: "empty.txt", ContentType: "text/plain", Body: bytes.NewReader(nil),
	})
	if !errors.Is(err, entity.ErrExecutionUploadEmpty) {
		t.Fatalf("an empty artifact answered %v", err)
	}

	var unnamed entity.ValidationError
	if _, err := h.service.SaveArtifact(context.Background(), h.execution.ID, service.ArtifactUpload{
		Name: "  ", ContentType: "text/plain", Body: bytes.NewReader([]byte("something")),
	}); !errors.As(err, &unnamed) {
		t.Fatalf("an artifact with no name answered %v", err)
	}

	if len(h.artifacts) != 0 {
		t.Fatalf("%d artifacts were recorded for an upload that carried nothing", len(h.artifacts))
	}
}

func TestABatchRefusedAtATakenPositionLeavesNothingInStorage(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	if _, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(1, "first"),
	}); err != nil {
		t.Fatalf("send a batch of output: %v", err)
	}

	kept := h.chunks[0].ObjectKey

	if _, err := h.service.AppendLogs(ctx, h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(1, "second"),
	}); !errors.Is(err, entity.ErrExecutionChunkConflict) {
		t.Fatalf("a second batch at position 1 answered %v", err)
	}

	if !h.stored(t, kept) {
		t.Fatal("the batch that was accepted lost its object when a later one was refused")
	}

	if left := h.storedBatches(t); left != 1 {
		t.Fatalf(
			"%d objects are in storage against 1 batch on record; a refused upload left one "+
				"behind that nothing will ever read or sweep",
			left,
		)
	}
}

func TestABatchWithoutAPositionInItsStreamIsRefusedByName(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.AppendLogs(context.Background(), h.execution.ID, service.LogBatch{
		Sequence: 0, Entries: h.logs(1, "nowhere in particular"),
	})
	if !errors.Is(err, entity.ErrExecutionSequenceInvalid) {
		t.Fatalf("a batch with no position answered %v", err)
	}
}

func TestABatchOfMoreEntriesThanTheServerTakesAtOnceIsRefusedByName(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.AppendLogs(context.Background(), h.execution.ID, service.LogBatch{
		Sequence: 1, Entries: h.logs(entity.ExecutionChunkMaxEntries+1, "one line"),
	})
	if !errors.Is(err, entity.ErrExecutionUploadCrowded) {
		t.Fatalf("an overcrowded batch answered %v", err)
	}
}

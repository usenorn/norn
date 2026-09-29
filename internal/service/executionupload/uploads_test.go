package executionupload_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func diff() service.ArtifactUpload {
	return service.ArtifactUpload{
		Name:        "backend.diff",
		ContentType: "text/plain",
		Body:        bytes.NewReader([]byte("--- a/main.go\n+++ b/main.go\n")),
	}
}

func TestAMachineThatDoesNotHoldTheRunIsToldThereIsNoSuchRun(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.SaveArtifact(context.Background(), entity.NewExecutionID("01OTHER"), diff())
	if !errors.Is(err, entity.ErrExecutionNotFound) {
		t.Fatalf(
			"uploading against a run this machine does not hold answered %v; anything but "+
				"not-found lets a machine probe for runs it was never given",
			err,
		)
	}

	if len(h.artifacts) != 0 {
		t.Fatalf("%d artifacts were stored for a run the machine does not hold", len(h.artifacts))
	}
}

func TestAFinishedRunTakesNoMoreUploads(t *testing.T) {
	h := newHarness(t)
	h.execution.State = entity.ExecutionCompleted

	_, err := h.service.SaveArtifact(context.Background(), h.execution.ID, diff())
	if !errors.Is(err, entity.ErrExecutionFinished) {
		t.Fatalf("uploading against a finished run answered %v", err)
	}
}

func TestARunThatHasUploadedItsAllowanceIsRefusedByName(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	for index := range 8 {
		body := bytes.Repeat([]byte(uuid.NewString()), (uploadLimit/3)/36)

		_, err := h.service.SaveArtifact(ctx, h.execution.ID, service.ArtifactUpload{
			Name: "part.bin", ContentType: "application/octet-stream", Body: bytes.NewReader(body),
		})
		if err == nil {
			continue
		}

		if errors.Is(err, entity.ErrExecutionUploadExhausted) {
			return
		}

		t.Fatalf("artifact %d answered %v", index, err)
	}

	t.Fatal("a run uploaded past its allowance and nothing turned it down")
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

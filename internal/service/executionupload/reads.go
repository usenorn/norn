package executionupload

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func (s *executionUploadsService) Logs(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	page entity.ExecutionChunkPage,
) ([]entity.ExecutionLogChunk, error) {
	chunks, err := s.readable(ctx, workspaceID, executionID, entity.ExecutionStreamLogs, page)
	if err != nil {
		return nil, err
	}

	held := make([]entity.ExecutionLogChunk, 0, len(chunks))

	for _, chunk := range chunks {
		var entries []entity.ExecutionLogEntry

		if err := s.decode(ctx, chunk, &entries); err != nil {
			return nil, err
		}

		held = append(held, entity.ExecutionLogChunk{ExecutionChunk: chunk, Entries: entries})
	}

	return held, nil
}

func (s *executionUploadsService) Transcript(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	page entity.ExecutionChunkPage,
) ([]entity.ExecutionTranscriptChunk, error) {
	chunks, err := s.readable(ctx, workspaceID, executionID, entity.ExecutionStreamTranscript, page)
	if err != nil {
		return nil, err
	}

	held := make([]entity.ExecutionTranscriptChunk, 0, len(chunks))

	for _, chunk := range chunks {
		var entries []entity.ExecutionTranscriptEntry

		if err := s.decode(ctx, chunk, &entries); err != nil {
			return nil, err
		}

		held = append(held, entity.ExecutionTranscriptChunk{ExecutionChunk: chunk, Entries: entries})
	}

	return held, nil
}

func (s *executionUploadsService) Artifacts(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) ([]entity.ExecutionArtifact, error) {
	execution, err := s.executions.Visible(ctx, workspaceID, executionID)
	if err != nil {
		return nil, err
	}

	return s.uploads.ListArtifacts(ctx, execution.ID)
}

func (s *executionUploadsService) readable(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	stream entity.ExecutionStream,
	page entity.ExecutionChunkPage,
) ([]entity.ExecutionChunk, error) {
	execution, err := s.executions.Visible(ctx, workspaceID, executionID)
	if err != nil {
		return nil, err
	}

	return s.uploads.ListChunks(ctx, execution.ID, stream, page)
}

// decode treats a chunk whose object has already gone as an empty one: retention removes the
// object before the row, so a sweep interrupted between the two must not turn the screen that
// reads it into a failure.
func (s *executionUploadsService) decode(
	ctx context.Context,
	chunk entity.ExecutionChunk,
	into any,
) error {
	object, err := s.blobs.Open(ctx, chunk.ObjectKey)
	if err != nil {
		if errors.Is(err, entity.ErrBlobNotFound) {
			return nil
		}

		return fmt.Errorf("read %s: %w", chunk.ObjectKey, err)
	}

	defer func() { _ = object.Close() }()

	unpacked, err := gzip.NewReader(object)
	if err != nil {
		return fmt.Errorf("read %s: %w", chunk.ObjectKey, err)
	}

	defer func() { _ = unpacked.Close() }()

	payload, err := io.ReadAll(io.LimitReader(unpacked, s.cfg.MaxChunkBytes))
	if err != nil {
		return fmt.Errorf("read %s: %w", chunk.ObjectKey, err)
	}

	if err := json.Unmarshal(payload, into); err != nil {
		return fmt.Errorf("read %s: %w", chunk.ObjectKey, err)
	}

	return nil
}

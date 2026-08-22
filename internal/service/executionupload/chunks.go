package executionupload

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type batch struct {
	stream   entity.ExecutionStream
	sequence int64
	payload  []byte
	entries  int
	firstAt  time.Time
	lastAt   time.Time
}

func (s *executionUploadsService) AppendLogs(
	ctx context.Context,
	executionID string,
	incoming service.LogBatch,
) (service.ExecutionReceipt, error) {
	execution, err := s.uploading(ctx, executionID)
	if err != nil {
		return service.ExecutionReceipt{}, err
	}

	if err := s.keeping(ctx, execution.WorkspaceID, entity.ExecutionStreamLogs); err != nil {
		return service.ExecutionReceipt{}, err
	}

	received := time.Now().UTC()

	if err := admissible(incoming.Sequence, len(incoming.Entries)); err != nil {
		return service.ExecutionReceipt{}, err
	}

	stamps := make([]time.Time, 0, len(incoming.Entries))

	for index, entry := range incoming.Entries {
		if entry.At.IsZero() {
			entry.At = received
		}

		entry.At = entry.At.UTC()
		entry.Source = text(entry.Source, entity.ExecutionEntrySourceMaxLen)
		entry.Stream = text(entry.Stream, entity.ExecutionEntryTypeMaxLen)

		incoming.Entries[index] = entry
		stamps = append(stamps, entry.At)
	}

	payload, err := canonical(incoming.Entries)
	if err != nil {
		return service.ExecutionReceipt{}, err
	}

	first, last := span(stamps, received)

	return s.store(ctx, execution, batch{
		stream:   entity.ExecutionStreamLogs,
		sequence: incoming.Sequence,
		payload:  payload,
		entries:  len(incoming.Entries),
		firstAt:  first,
		lastAt:   last,
	}, received)
}

func (s *executionUploadsService) AppendTranscript(
	ctx context.Context,
	executionID string,
	incoming service.TranscriptBatch,
) (service.ExecutionReceipt, error) {
	execution, err := s.uploading(ctx, executionID)
	if err != nil {
		return service.ExecutionReceipt{}, err
	}

	if err := s.keeping(ctx, execution.WorkspaceID, entity.ExecutionStreamTranscript); err != nil {
		return service.ExecutionReceipt{}, err
	}

	received := time.Now().UTC()

	if err := admissible(incoming.Sequence, len(incoming.Entries)); err != nil {
		return service.ExecutionReceipt{}, err
	}

	stamps := make([]time.Time, 0, len(incoming.Entries))

	for index, entry := range incoming.Entries {
		if entry.At.IsZero() {
			entry.At = received
		}

		entry.At = entry.At.UTC()
		entry.Type = text(entry.Type, entity.ExecutionEntryTypeMaxLen)

		incoming.Entries[index] = entry
		stamps = append(stamps, entry.At)
	}

	payload, err := canonical(incoming.Entries)
	if err != nil {
		return service.ExecutionReceipt{}, err
	}

	first, last := span(stamps, received)

	return s.store(ctx, execution, batch{
		stream:   entity.ExecutionStreamTranscript,
		sequence: incoming.Sequence,
		payload:  payload,
		entries:  len(incoming.Entries),
		firstAt:  first,
		lastAt:   last,
	}, received)
}

func (s *executionUploadsService) store(
	ctx context.Context,
	execution entity.Execution,
	held batch,
	received time.Time,
) (service.ExecutionReceipt, error) {
	if int64(len(held.payload)) > s.cfg.MaxChunkBytes {
		return service.ExecutionReceipt{}, entity.ExecutionUploadTooLargeError{
			SizeBytes: int64(len(held.payload)),
			MaxBytes:  s.cfg.MaxChunkBytes,
		}
	}

	digest, stored, err := compress(held.payload)
	if err != nil {
		return service.ExecutionReceipt{}, err
	}

	if err := s.affordable(ctx, execution.ID, int64(len(stored))); err != nil {
		return service.ExecutionReceipt{}, err
	}

	key := entity.ExecutionChunkKey(execution.WorkspaceID, execution.ID, held.stream, digest)

	if err := s.blobs.Put(
		ctx, key, chunkContentType, bytes.NewReader(stored), int64(len(stored)),
	); err != nil {
		return service.ExecutionReceipt{}, fmt.Errorf("store an execution batch: %w", err)
	}

	appended, err := s.uploads.AppendChunk(ctx, entity.ExecutionChunk{
		ExecutionID: execution.ID,
		WorkspaceID: execution.WorkspaceID,
		Stream:      held.stream,
		Sequence:    held.sequence,
		Digest:      digest,
		Bytes:       int64(len(stored)),
		Entries:     held.entries,
		ObjectKey:   key,
		FirstAt:     held.firstAt,
		LastAt:      held.lastAt,
		ReceivedAt:  received,
	})
	if err != nil {
		// A batch already on record is the object that is already there, because the key is the
		// digest of what it holds. Anything else leaves an object nothing points at.
		if errors.Is(err, entity.ErrExecutionUploadRecorded) {
			already, err := s.uploads.Chunk(ctx, execution.ID, held.stream, digest)
			if err != nil {
				return service.ExecutionReceipt{}, err
			}

			return service.ExecutionReceipt{Chunk: already, Duplicate: true}, nil
		}

		s.discard(ctx, key)

		return service.ExecutionReceipt{}, err
	}

	return service.ExecutionReceipt{Chunk: appended}, nil
}

func admissible(sequence int64, entries int) error {
	switch {
	case sequence < 1:
		return entity.ErrExecutionSequenceInvalid
	case entries == 0:
		return entity.ErrExecutionUploadEmpty
	case entries > entity.ExecutionChunkMaxEntries:
		return entity.ErrExecutionUploadCrowded
	default:
		return nil
	}
}

func compress(payload []byte) (string, []byte, error) {
	sum := sha256.Sum256(payload)

	var packed bytes.Buffer

	writer := gzip.NewWriter(&packed)

	if _, err := writer.Write(payload); err != nil {
		return "", nil, fmt.Errorf("compress an execution batch: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", nil, fmt.Errorf("compress an execution batch: %w", err)
	}

	return hex.EncodeToString(sum[:]), packed.Bytes(), nil
}

package executionupload_test

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	blobgrantrepo "github.com/usenorn/norn/internal/repository/blobgrant"
	policyrepo "github.com/usenorn/norn/internal/repository/executionpolicy"
	uploadrepo "github.com/usenorn/norn/internal/repository/executionupload"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	uploadsvc "github.com/usenorn/norn/internal/service/executionupload"
	runnersvc "github.com/usenorn/norn/internal/service/runner"
)

const (
	chunkLimit  = 4096
	uploadLimit = 32 << 10
	batchSuffix = ".json.gz"
)

type harness struct {
	uploads    *uploadrepo.MockExecutionUpload
	policies   *policyrepo.MockExecutionPolicy
	blobs      repository.Blob
	runners    *runnersvc.MockRunners
	executions *executionsvc.MockExecutions
	authorizer *authorizersvc.MockAuthorizer
	service    service.ExecutionUploads
	root       string

	workspaceID uuid.UUID
	runner      entity.Runner
	execution   entity.Execution

	chunks    []entity.ExecutionChunk
	artifacts []entity.ExecutionArtifact
	policy    entity.WorkspaceExecutionPolicy
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)
	workspaceID := uuid.New()

	root := t.TempDir()

	blobs, err := blobrepo.New(
		config.Storage{Backend: config.StorageBackendFilesystem, Root: root},
		config.Attachments{MaxFileBytes: uploadLimit, UploadTTL: time.Minute, LinkTTL: time.Minute},
		blobgrantrepo.NewMockBlobGrant(ctrl),
	)
	if err != nil {
		t.Fatalf("open a store to keep uploads in: %v", err)
	}

	h := &harness{
		uploads:     uploadrepo.NewMockExecutionUpload(ctrl),
		policies:    policyrepo.NewMockExecutionPolicy(ctrl),
		blobs:       blobs,
		runners:     runnersvc.NewMockRunners(ctrl),
		executions:  executionsvc.NewMockExecutions(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		root:        root,
		workspaceID: workspaceID,
		runner:      entity.Runner{ID: uuid.New(), WorkspaceID: workspaceID, AgentID: uuid.New()},
		policy:      entity.WorkspaceExecutionPolicy{WorkspaceID: workspaceID},
	}

	h.execution = entity.Execution{
		ID:          entity.NewExecutionID("01ABCDEF"),
		WorkspaceID: workspaceID,
		IssueID:     uuid.New(),
		RunnerID:    h.runner.ID,
		State:       entity.ExecutionRunning,
	}

	h.service = uploadsvc.New(
		h.uploads,
		h.policies,
		h.blobs,
		h.runners,
		h.executions,
		h.authorizer,
		config.Executions{
			MaxChunkBytes:    chunkLimit,
			MaxArtifactBytes: uploadLimit,
			MaxUploadBytes:   uploadLimit,
			UploadRetention:  90 * 24 * time.Hour,
			RetentionBatch:   10,
		},
		config.Attachments{LinkTTL: time.Minute},
	)

	h.expectCallingRunner()
	h.expectStore()

	return h
}

func (h *harness) expectCallingRunner() {
	h.runners.EXPECT().Self(gomock.Any()).Return(h.runner, nil).AnyTimes()

	h.executions.EXPECT().
		Held(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, runner entity.Runner, executionID string) (entity.Execution, error) {
			if runner.ID != h.execution.RunnerID || executionID != h.execution.ID {
				return entity.Execution{}, entity.ErrExecutionNotFound
			}

			return h.execution, nil
		}).
		AnyTimes()

	h.executions.EXPECT().
		Visible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID uuid.UUID, executionID string) (entity.Execution, error) {
			if workspaceID != h.workspaceID || executionID != h.execution.ID {
				return entity.Execution{}, entity.ErrExecutionNotFound
			}

			return h.execution, nil
		}).
		AnyTimes()
}

// expectStore keeps the rows this service writes in memory and enforces the two unique indexes the
// schema declares, because idempotency is what those indexes buy and a mock that always accepts
// would let a replay through without anything noticing.
func (h *harness) expectStore() {
	h.policies.EXPECT().
		Policy(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID uuid.UUID) (entity.WorkspaceExecutionPolicy, error) {
			return h.policy, nil
		}).
		AnyTimes()

	h.policies.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, policy entity.WorkspaceExecutionPolicy) (entity.WorkspaceExecutionPolicy, error) {
			h.policy = policy

			return policy, nil
		}).
		AnyTimes()

	h.uploads.EXPECT().
		AppendChunk(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, chunk entity.ExecutionChunk) (entity.ExecutionChunk, error) {
			for _, held := range h.chunks {
				if held.ExecutionID != chunk.ExecutionID || held.Stream != chunk.Stream {
					continue
				}

				if held.Digest == chunk.Digest {
					return entity.ExecutionChunk{}, entity.ErrExecutionUploadRecorded
				}

				if held.Sequence == chunk.Sequence {
					return entity.ExecutionChunk{}, entity.ErrExecutionChunkConflict
				}
			}

			chunk.ID = uuid.New()
			h.chunks = append(h.chunks, chunk)

			return chunk, nil
		}).
		AnyTimes()

	h.uploads.EXPECT().
		Chunk(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, executionID string, stream entity.ExecutionStream, digest string,
		) (entity.ExecutionChunk, error) {
			for _, held := range h.chunks {
				if held.ExecutionID == executionID && held.Stream == stream && held.Digest == digest {
					return held, nil
				}
			}

			return entity.ExecutionChunk{}, entity.ErrExecutionNotFound
		}).
		AnyTimes()

	h.uploads.EXPECT().
		ListChunks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			executionID string,
			stream entity.ExecutionStream,
			page entity.ExecutionChunkPage,
		) ([]entity.ExecutionChunk, error) {
			page = page.Normalized()
			found := make([]entity.ExecutionChunk, 0, len(h.chunks))

			for _, held := range h.chunks {
				if held.ExecutionID == executionID &&
					held.Stream == stream &&
					held.Sequence > page.After {
					found = append(found, held)
				}
			}

			if len(found) > page.Limit {
				found = found[:page.Limit]
			}

			return found, nil
		}).
		AnyTimes()

	h.uploads.EXPECT().
		UploadedBytes(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, executionID string) (int64, error) {
			var stored int64

			for _, held := range h.chunks {
				if held.ExecutionID == executionID {
					stored += held.Bytes
				}
			}

			for _, held := range h.artifacts {
				if held.ExecutionID == executionID {
					stored += held.Bytes
				}
			}

			return stored, nil
		}).
		AnyTimes()

	h.uploads.EXPECT().
		Cursors(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, executionID string) ([]entity.ExecutionStreamCursor, error) {
			cursors := map[entity.ExecutionStream]entity.ExecutionStreamCursor{}

			for _, held := range h.chunks {
				if held.ExecutionID != executionID {
					continue
				}

				cursor := cursors[held.Stream]
				cursor.Stream = held.Stream
				cursor.Chunks++
				cursor.Entries += int64(held.Entries)
				cursor.Bytes += held.Bytes

				if held.Sequence > cursor.LastSequence {
					cursor.LastSequence = held.Sequence
				}

				cursors[held.Stream] = cursor
			}

			found := make([]entity.ExecutionStreamCursor, 0, len(cursors))
			for _, stream := range entity.ExecutionStreams() {
				if cursor, ok := cursors[stream]; ok {
					found = append(found, cursor)
				}
			}

			return found, nil
		}).
		AnyTimes()

	h.uploads.EXPECT().
		DropChunk(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, chunkID uuid.UUID) error {
			kept := make([]entity.ExecutionChunk, 0, len(h.chunks))

			for _, held := range h.chunks {
				if held.ID != chunkID {
					kept = append(kept, held)
				}
			}

			h.chunks = kept

			return nil
		}).
		AnyTimes()

	h.uploads.EXPECT().
		SaveArtifact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, artifact entity.ExecutionArtifact) (entity.ExecutionArtifact, error) {
			for _, held := range h.artifacts {
				if held.ExecutionID == artifact.ExecutionID && held.Digest == artifact.Digest {
					return entity.ExecutionArtifact{}, entity.ErrExecutionUploadRecorded
				}
			}

			artifact.CreatedAt = time.Now().UTC()
			h.artifacts = append(h.artifacts, artifact)

			return artifact, nil
		}).
		AnyTimes()

	h.uploads.EXPECT().
		ArtifactByDigest(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, executionID, digest string) (entity.ExecutionArtifact, error) {
			for _, held := range h.artifacts {
				if held.ExecutionID == executionID && held.Digest == digest {
					return held, nil
				}
			}

			return entity.ExecutionArtifact{}, entity.ErrExecutionArtifactNotFound
		}).
		AnyTimes()

	h.uploads.EXPECT().
		ListArtifacts(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, executionID string) ([]entity.ExecutionArtifact, error) {
			found := make([]entity.ExecutionArtifact, 0, len(h.artifacts))

			for _, held := range h.artifacts {
				if held.ExecutionID == executionID {
					found = append(found, held)
				}
			}

			return found, nil
		}).
		AnyTimes()

	h.uploads.EXPECT().
		ExpiredChunks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, now time.Time, fallbackDays, limit int,
		) ([]entity.ExecutionChunk, error) {
			days := fallbackDays
			if h.policy.UploadRetentionDays > 0 {
				days = h.policy.UploadRetentionDays
			}

			cutoff := now.AddDate(0, 0, -days)
			found := make([]entity.ExecutionChunk, 0, len(h.chunks))

			for _, held := range h.chunks {
				if held.ReceivedAt.Before(cutoff) && len(found) < limit {
					found = append(found, held)
				}
			}

			return found, nil
		}).
		AnyTimes()
}

func (h *harness) expectPolicyDecision() {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{}, nil).
		AnyTimes()
}

func (h *harness) stored(t *testing.T, key string) bool {
	t.Helper()

	if _, err := h.blobs.Stat(context.Background(), key); err != nil {
		return false
	}

	return true
}

func (h *harness) storedBatches(t *testing.T) int {
	t.Helper()

	found := 0

	if err := filepath.WalkDir(h.root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && strings.HasSuffix(entry.Name(), batchSuffix) {
			found++
		}

		return nil
	}); err != nil {
		t.Fatalf("walk the store: %v", err)
	}

	return found
}

func (h *harness) logs(count int, text string) []entity.ExecutionLogEntry {
	entries := make([]entity.ExecutionLogEntry, 0, count)

	for index := range count {
		entries = append(entries, entity.ExecutionLogEntry{
			At:     time.Date(2026, 8, 22, 9, index, 0, 0, time.UTC),
			Stream: "stdout",
			Source: "build",
			Text:   text,
		})
	}

	return entries
}

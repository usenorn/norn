package executionupload_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	blobgrantrepo "github.com/usenorn/norn/internal/repository/blobgrant"
	uploadrepo "github.com/usenorn/norn/internal/repository/executionupload"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	uploadsvc "github.com/usenorn/norn/internal/service/executionupload"
	runnersvc "github.com/usenorn/norn/internal/service/runner"
)

const uploadLimit = 32 << 10

type harness struct {
	uploads    *uploadrepo.MockExecutionUpload
	blobs      repository.Blob
	runners    *runnersvc.MockRunners
	executions *executionsvc.MockExecutions
	authorizer *authorizersvc.MockAuthorizer
	service    service.ExecutionUploads
	root       string

	workspaceID uuid.UUID
	runner      entity.Runner
	execution   entity.Execution

	artifacts []entity.ExecutionArtifact
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
		blobs:       blobs,
		runners:     runnersvc.NewMockRunners(ctrl),
		executions:  executionsvc.NewMockExecutions(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		root:        root,
		workspaceID: workspaceID,
		runner:      entity.Runner{ID: uuid.New(), WorkspaceID: workspaceID, AgentID: uuid.New()},
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
		h.blobs,
		h.runners,
		h.executions,
		h.authorizer,
		config.Executions{
			MaxArtifactBytes: uploadLimit,
			MaxUploadBytes:   uploadLimit,
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

func (h *harness) expectStore() {
	h.uploads.EXPECT().
		UploadedBytes(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, executionID string) (int64, error) {
			var stored int64

			for _, held := range h.artifacts {
				if held.ExecutionID == executionID {
					stored += held.Bytes
				}
			}

			return stored, nil
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
}

func (h *harness) stored(t *testing.T, key string) bool {
	t.Helper()

	if _, err := h.blobs.Stat(context.Background(), key); err != nil {
		return false
	}

	return true
}

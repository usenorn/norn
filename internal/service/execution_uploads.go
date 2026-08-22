package service

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_uploads.go -destination=executionupload/mock_execution_uploads.go -package=executionupload -mock_names=ExecutionUploads=MockExecutionUploads

type LogBatch struct {
	Sequence int64
	Entries  []entity.ExecutionLogEntry
}

type TranscriptBatch struct {
	Sequence int64
	Entries  []entity.ExecutionTranscriptEntry
}

type ArtifactUpload struct {
	Name        string
	ContentType string
	Body        io.Reader
}

type ExecutionReceipt struct {
	Chunk     entity.ExecutionChunk
	Duplicate bool
}

type ArtifactReceipt struct {
	Artifact  entity.ExecutionArtifact
	Duplicate bool
}

type ExecutionUploads interface {
	AppendLogs(ctx context.Context, executionID string, batch LogBatch) (ExecutionReceipt, error)
	AppendTranscript(
		ctx context.Context,
		executionID string,
		batch TranscriptBatch,
	) (ExecutionReceipt, error)
	SaveArtifact(
		ctx context.Context,
		executionID string,
		upload ArtifactUpload,
	) (ArtifactReceipt, error)
	Cursors(ctx context.Context, executionID string) ([]entity.ExecutionStreamCursor, error)
	Telemetry(ctx context.Context) (entity.TelemetryMode, error)

	Logs(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
		page entity.ExecutionChunkPage,
	) ([]entity.ExecutionLogChunk, error)
	Transcript(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
		page entity.ExecutionChunkPage,
	) ([]entity.ExecutionTranscriptChunk, error)
	Artifacts(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
	) ([]entity.ExecutionArtifact, error)
	ArtifactContent(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
		artifactID uuid.UUID,
	) (string, error)

	Policy(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceExecutionPolicy, error)
	SetPolicy(
		ctx context.Context,
		policy entity.WorkspaceExecutionPolicy,
	) (entity.WorkspaceExecutionPolicy, error)

	Prune(ctx context.Context) error
}

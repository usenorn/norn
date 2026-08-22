package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_upload.go -destination=executionupload/mock_execution_upload.go -package=executionupload -mock_names=ExecutionUpload=MockExecutionUpload

type ExecutionUpload interface {
	AppendChunk(ctx context.Context, chunk entity.ExecutionChunk) (entity.ExecutionChunk, error)
	Chunk(
		ctx context.Context,
		executionID string,
		stream entity.ExecutionStream,
		digest string,
	) (entity.ExecutionChunk, error)
	ListChunks(
		ctx context.Context,
		executionID string,
		stream entity.ExecutionStream,
		page entity.ExecutionChunkPage,
	) ([]entity.ExecutionChunk, error)
	Cursors(ctx context.Context, executionID string) ([]entity.ExecutionStreamCursor, error)
	UploadedBytes(ctx context.Context, executionID string) (int64, error)
	ExpiredChunks(
		ctx context.Context,
		now time.Time,
		fallbackDays, limit int,
	) ([]entity.ExecutionChunk, error)
	DropChunk(ctx context.Context, chunkID uuid.UUID) error

	SaveArtifact(ctx context.Context, artifact entity.ExecutionArtifact) (entity.ExecutionArtifact, error)
	Artifact(ctx context.Context, executionID string, artifactID uuid.UUID) (entity.ExecutionArtifact, error)
	ArtifactByDigest(ctx context.Context, executionID, digest string) (entity.ExecutionArtifact, error)
	ListArtifacts(ctx context.Context, executionID string) ([]entity.ExecutionArtifact, error)
}

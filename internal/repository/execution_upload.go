package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_upload.go -destination=executionupload/mock_execution_upload.go -package=executionupload -mock_names=ExecutionUpload=MockExecutionUpload

type ExecutionUpload interface {
	UploadedBytes(ctx context.Context, executionID string) (int64, error)
	SaveArtifact(ctx context.Context, artifact entity.ExecutionArtifact) (entity.ExecutionArtifact, error)
	Artifact(ctx context.Context, executionID string, artifactID uuid.UUID) (entity.ExecutionArtifact, error)
	ArtifactByDigest(ctx context.Context, executionID, digest string) (entity.ExecutionArtifact, error)
	ListArtifacts(ctx context.Context, executionID string) ([]entity.ExecutionArtifact, error)
}

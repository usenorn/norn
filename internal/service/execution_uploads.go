package service

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_uploads.go -destination=executionupload/mock_execution_uploads.go -package=executionupload -mock_names=ExecutionUploads=MockExecutionUploads

type ArtifactUpload struct {
	Name        string
	ContentType string
	Body        io.Reader
}

type ArtifactReceipt struct {
	Artifact  entity.ExecutionArtifact
	Duplicate bool
}

type ExecutionUploads interface {
	SaveArtifact(
		ctx context.Context,
		executionID string,
		upload ArtifactUpload,
	) (ArtifactReceipt, error)
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
}

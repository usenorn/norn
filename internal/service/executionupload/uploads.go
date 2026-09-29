package executionupload

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type executionUploadsService struct {
	uploads     repository.ExecutionUpload
	blobs       repository.Blob
	runners     service.Runners
	executions  service.Executions
	authorizer  service.Authorizer
	cfg         config.Executions
	attachments config.Attachments
}

func New(
	uploads repository.ExecutionUpload,
	blobs repository.Blob,
	runners service.Runners,
	executions service.Executions,
	authorizer service.Authorizer,
	cfg config.Executions,
	attachments config.Attachments,
) service.ExecutionUploads {
	return &executionUploadsService{
		uploads:     uploads,
		blobs:       blobs,
		runners:     runners,
		executions:  executions,
		authorizer:  authorizer,
		cfg:         cfg,
		attachments: attachments,
	}
}

func (s *executionUploadsService) uploading(
	ctx context.Context,
	executionID string,
) (entity.Execution, error) {
	runner, err := s.runners.Self(ctx)
	if err != nil {
		return entity.Execution{}, err
	}

	execution, err := s.executions.Held(ctx, runner, executionID)
	if err != nil {
		return entity.Execution{}, err
	}

	if execution.Finished() {
		return entity.Execution{}, entity.ErrExecutionFinished
	}

	return execution, nil
}

func (s *executionUploadsService) affordable(
	ctx context.Context,
	executionID string,
	size int64,
) error {
	stored, err := s.uploads.UploadedBytes(ctx, executionID)
	if err != nil {
		return err
	}

	if stored+size > s.cfg.MaxUploadBytes {
		return entity.ExecutionUploadExhaustedError{
			SizeBytes:     size,
			UploadedBytes: stored,
			MaxBytes:      s.cfg.MaxUploadBytes,
		}
	}

	return nil
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

func text(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if utf8.RuneCountInString(trimmed) <= max {
		return trimmed
	}

	return string([]rune(trimmed)[:max])
}

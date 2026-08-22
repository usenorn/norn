package executionupload

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const chunkContentType = "application/gzip"

type executionUploadsService struct {
	uploads     repository.ExecutionUpload
	policies    repository.ExecutionPolicy
	blobs       repository.Blob
	runners     service.Runners
	executions  service.Executions
	authorizer  service.Authorizer
	cfg         config.Executions
	attachments config.Attachments
}

func New(
	uploads repository.ExecutionUpload,
	policies repository.ExecutionPolicy,
	blobs repository.Blob,
	runners service.Runners,
	executions service.Executions,
	authorizer service.Authorizer,
	cfg config.Executions,
	attachments config.Attachments,
) service.ExecutionUploads {
	return &executionUploadsService{
		uploads:     uploads,
		policies:    policies,
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

func (s *executionUploadsService) keeping(
	ctx context.Context,
	workspaceID uuid.UUID,
	stream entity.ExecutionStream,
) error {
	policy, err := s.effective(ctx, workspaceID)
	if err != nil {
		return err
	}

	if !policy.Telemetry.Keeps(stream) {
		return entity.ErrExecutionTelemetryMinimal
	}

	return nil
}

func (s *executionUploadsService) effective(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.WorkspaceExecutionPolicy, error) {
	policy, err := s.policies.Policy(ctx, workspaceID)
	if err != nil {
		return entity.WorkspaceExecutionPolicy{}, err
	}

	return policy.Normalised(s.cfg.UploadRetention), nil
}

func (s *executionUploadsService) Cursors(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionStreamCursor, error) {
	if _, err := s.uploading(ctx, executionID); err != nil {
		return nil, err
	}

	return s.uploads.Cursors(ctx, executionID)
}

// Telemetry answers the calling machine, and only about its own workspace, so a runner learns to
// send summaries without holding any scope over the workspace it runs for.
func (s *executionUploadsService) Telemetry(ctx context.Context) (entity.TelemetryMode, error) {
	runner, err := s.runners.Self(ctx)
	if err != nil {
		return "", err
	}

	policy, err := s.effective(ctx, runner.WorkspaceID)
	if err != nil {
		return "", err
	}

	return policy.Telemetry, nil
}

func canonical(entries any) ([]byte, error) {
	encoded, err := json.Marshal(entries)
	if err != nil {
		return nil, errors.Join(entity.ErrExecutionUploadEmpty, err)
	}

	return encoded, nil
}

func text(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if utf8.RuneCountInString(trimmed) <= max {
		return trimmed
	}

	return string([]rune(trimmed)[:max])
}

func span(stamps []time.Time, fallback time.Time) (time.Time, time.Time) {
	first, last := fallback, fallback

	for index, stamp := range stamps {
		if index == 0 || stamp.Before(first) {
			first = stamp
		}

		if index == 0 || stamp.After(last) {
			last = stamp
		}
	}

	return first, last
}

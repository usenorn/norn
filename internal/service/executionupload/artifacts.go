package executionupload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

type measured struct {
	reader io.Reader
	digest hash.Hash
	limit  int64
	read   int64
}

func (m *measured) Read(into []byte) (int, error) {
	read, err := m.reader.Read(into)
	if read > 0 {
		m.read += int64(read)

		if m.read > m.limit {
			return 0, entity.ExecutionUploadTooLargeError{SizeBytes: m.read, MaxBytes: m.limit}
		}

		if _, err := m.digest.Write(into[:read]); err != nil {
			return 0, err
		}
	}

	return read, err
}

func (s *executionUploadsService) SaveArtifact(
	ctx context.Context,
	executionID string,
	upload service.ArtifactUpload,
) (service.ArtifactReceipt, error) {
	execution, err := s.uploading(ctx, executionID)
	if err != nil {
		return service.ArtifactReceipt{}, err
	}

	name := text(upload.Name, entity.ExecutionArtifactNameMaxLen)
	if err := entity.NewValidationError(
		entity.ValidateExecutionArtifactName("file", name),
	); err != nil {
		return service.ArtifactReceipt{}, err
	}

	if err := s.affordable(ctx, execution.ID, 0); err != nil {
		return service.ArtifactReceipt{}, err
	}

	artifactID := uuid.New()
	key := entity.ExecutionArtifactKey(execution.WorkspaceID, execution.ID, artifactID)
	contentType := text(upload.ContentType, entity.ExecutionContentTypeMaxLen)

	if contentType == "" {
		contentType = entity.ExecutionArtifactGenericType
	}

	counted := &measured{
		reader: upload.Body,
		digest: sha256.New(),
		limit:  s.cfg.MaxArtifactBytes,
	}

	if err := s.blobs.Put(ctx, key, contentType, counted, -1); err != nil {
		s.discard(ctx, key)

		// A store wraps a read failure in whatever type it likes, so the count is what says why
		// the write stopped rather than the error that came back through the driver.
		if counted.read >= s.cfg.MaxArtifactBytes {
			return service.ArtifactReceipt{}, entity.ExecutionUploadTooLargeError{
				SizeBytes: counted.read,
				MaxBytes:  s.cfg.MaxArtifactBytes,
			}
		}

		return service.ArtifactReceipt{}, err
	}

	if counted.read == 0 {
		s.discard(ctx, key)

		return service.ArtifactReceipt{}, entity.ErrExecutionUploadEmpty
	}

	if err := s.affordable(ctx, execution.ID, counted.read); err != nil {
		s.discard(ctx, key)

		return service.ArtifactReceipt{}, err
	}

	saved, err := s.uploads.SaveArtifact(ctx, entity.ExecutionArtifact{
		ID:          artifactID,
		ExecutionID: execution.ID,
		WorkspaceID: execution.WorkspaceID,
		Name:        name,
		ContentType: contentType,
		Bytes:       counted.read,
		Digest:      hex.EncodeToString(counted.digest.Sum(nil)),
		ObjectKey:   key,
	})
	if err != nil {
		s.discard(ctx, key)

		if errors.Is(err, entity.ErrExecutionUploadRecorded) {
			already, err := s.uploads.ArtifactByDigest(
				ctx, execution.ID, hex.EncodeToString(counted.digest.Sum(nil)),
			)
			if err != nil {
				return service.ArtifactReceipt{}, err
			}

			return service.ArtifactReceipt{Artifact: already, Duplicate: true}, nil
		}

		return service.ArtifactReceipt{}, err
	}

	return service.ArtifactReceipt{Artifact: saved}, nil
}

func (s *executionUploadsService) discard(ctx context.Context, key string) {
	if err := s.blobs.Delete(ctx, key); err != nil && !errors.Is(err, entity.ErrBlobNotFound) {
		logging.From(ctx).WarnContext(
			ctx, "removing an execution upload that was not recorded failed",
			"object_key", key, "error", err.Error(),
		)
	}
}

func (s *executionUploadsService) ArtifactContent(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	artifactID uuid.UUID,
) (string, error) {
	execution, err := s.executions.Visible(ctx, workspaceID, executionID)
	if err != nil {
		return "", err
	}

	artifact, err := s.uploads.Artifact(ctx, execution.ID, artifactID)
	if err != nil {
		return "", err
	}

	link, err := s.blobs.PresignGet(ctx, artifact.ObjectKey, entity.ServeSpec{
		ContentType: entity.AttachmentServedType(artifact.ContentType),
		Disposition: entity.AttachmentDispositionAt,
		FileName:    artifact.Name,
	}, s.attachments.LinkTTL)
	if err != nil {
		return "", fmt.Errorf("link to an execution artifact: %w", err)
	}

	return link, nil
}

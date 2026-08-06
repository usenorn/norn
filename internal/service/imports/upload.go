package imports

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *importsService) Upload(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
	upload service.ImportUpload,
) (service.ImportFile, error) {
	if _, err := s.decide(ctx, workspaceID); err != nil {
		return service.ImportFile{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return service.ImportFile{}, err
	}

	if !run.Status.Configurable() {
		return service.ImportFile{}, entity.ErrImportStatusTransition
	}

	if field := entity.ValidateAttachmentName("file", upload.FileName); field.Code != "" {
		return service.ImportFile{}, entity.NewValidationError(field)
	}

	key := entity.ImportBlobKey(run.WorkspaceID, run.ID, upload.FileName)

	// The body arrives as a multipart part, which cannot be rewound, and the object store has
	// to read it twice: once to sign the payload and once to send it. The upload is capped
	// well below what a request may carry, so holding it is bounded and is the only way this
	// reaches storage at all.
	body, err := io.ReadAll(upload.Body)
	if err != nil {
		return service.ImportFile{}, fmt.Errorf("read uploaded file: %w", err)
	}

	if len(body) == 0 {
		return service.ImportFile{}, entity.NewValidationError(entity.FieldError{
			Field: "file",
			Code:  entity.ValidationCodeRequired,
		})
	}

	if err := s.blobs.Put(
		ctx, key, entity.AttachmentGenericType, bytes.NewReader(body), int64(len(body)),
	); err != nil {
		return service.ImportFile{}, err
	}

	return service.ImportFile{
		ObjectKey: key,
		FileName:  entity.AttachmentFileName(upload.FileName),
	}, nil
}

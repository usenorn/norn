package executionupload

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *executionUploadsService) Prune(ctx context.Context) error {
	expired, err := s.uploads.ExpiredChunks(
		ctx,
		time.Now().UTC(),
		entity.ExecutionRetentionDays(s.cfg.UploadRetention),
		s.cfg.RetentionBatch,
	)
	if err != nil {
		return err
	}

	failures := 0

	for _, chunk := range expired {
		if err := s.release(ctx, chunk); err != nil {
			failures++

			logging.From(ctx).WarnContext(
				ctx, "removing an aged execution batch failed",
				"execution_id", chunk.ExecutionID,
				"object_key", chunk.ObjectKey,
				"error", err.Error(),
			)
		}
	}

	if failures > 0 {
		return fmt.Errorf("remove %d of %d aged execution batches", failures, len(expired))
	}

	return nil
}

func (s *executionUploadsService) release(ctx context.Context, chunk entity.ExecutionChunk) error {
	if err := s.blobs.Delete(ctx, chunk.ObjectKey); err != nil &&
		!errors.Is(err, entity.ErrBlobNotFound) {
		return err
	}

	return s.uploads.DropChunk(ctx, chunk.ID)
}

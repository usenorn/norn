package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type BulkApplyHandler struct {
	operations service.BulkOperations
}

func NewBulkApplyHandler(operations service.BulkOperations) *BulkApplyHandler {
	return &BulkApplyHandler{operations: operations}
}

func (h *BulkApplyHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.BulkApplyPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode bulk apply payload: %w", err), asynq.SkipRetry)
	}

	if payload.BulkActionID == uuid.Nil || payload.WorkspaceID == uuid.Nil {
		return errors.Join(errors.New("bulk apply payload is incomplete"), asynq.SkipRetry)
	}

	return h.operations.Run(ctx, payload)
}

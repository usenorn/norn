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

type ImportRevertHandler struct {
	imports service.ImportRunner
}

func NewImportRevertHandler(imports service.ImportRunner) *ImportRevertHandler {
	return &ImportRevertHandler{imports: imports}
}

func (h *ImportRevertHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.ImportRevertPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode import revert payload: %w", err), asynq.SkipRetry)
	}

	if payload.ImportRunID == uuid.Nil || payload.WorkspaceID == uuid.Nil {
		return errors.Join(errors.New("import revert payload is incomplete"), asynq.SkipRetry)
	}

	return h.imports.RunRevert(ctx, payload)
}

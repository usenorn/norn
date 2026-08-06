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

type ImportExecuteHandler struct {
	imports service.ImportRunner
}

func NewImportExecuteHandler(imports service.ImportRunner) *ImportExecuteHandler {
	return &ImportExecuteHandler{imports: imports}
}

func (h *ImportExecuteHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.ImportExecutePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode import execute payload: %w", err), asynq.SkipRetry)
	}

	if payload.ImportRunID == uuid.Nil || payload.WorkspaceID == uuid.Nil {
		return errors.Join(errors.New("import execute payload is incomplete"), asynq.SkipRetry)
	}

	return h.imports.RunExecute(ctx, payload)
}

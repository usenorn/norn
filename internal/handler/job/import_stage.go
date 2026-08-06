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

type ImportStageHandler struct {
	imports service.ImportRunner
}

func NewImportStageHandler(imports service.ImportRunner) *ImportStageHandler {
	return &ImportStageHandler{imports: imports}
}

func (h *ImportStageHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.ImportStagePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode import stage payload: %w", err), asynq.SkipRetry)
	}

	if payload.ImportRunID == uuid.Nil || payload.WorkspaceID == uuid.Nil {
		return errors.Join(errors.New("import stage payload is incomplete"), asynq.SkipRetry)
	}

	return h.imports.RunStage(ctx, payload)
}

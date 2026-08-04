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

type WorkspacePurgeHandler struct {
	workspaces service.Workspaces
}

func NewWorkspacePurgeHandler(workspaces service.Workspaces) *WorkspacePurgeHandler {
	return &WorkspacePurgeHandler{workspaces: workspaces}
}

func (h *WorkspacePurgeHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.WorkspacePurgePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode workspace purge payload: %w", err), asynq.SkipRetry)
	}

	if payload.WorkspaceID == uuid.Nil {
		return errors.Join(errors.New("workspace purge payload is incomplete"), asynq.SkipRetry)
	}

	return h.workspaces.Purge(ctx, payload.WorkspaceID)
}

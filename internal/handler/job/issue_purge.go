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

type IssuePurgeHandler struct {
	issues service.Issues
}

func NewIssuePurgeHandler(issues service.Issues) *IssuePurgeHandler {
	return &IssuePurgeHandler{issues: issues}
}

func (h *IssuePurgeHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.IssuePurgePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode issue purge payload: %w", err), asynq.SkipRetry)
	}

	if payload.IssueID == uuid.Nil {
		return errors.Join(errors.New("issue purge payload is incomplete"), asynq.SkipRetry)
	}

	if err := h.issues.Purge(ctx, payload.IssueID); err != nil {
		if errors.Is(err, entity.ErrIssuePurgeNotDue) {
			return nil
		}

		return err
	}

	return nil
}

package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type SCMResumeHandler struct {
	sync service.SourceControlSync
}

func NewSCMResumeHandler(sync service.SourceControlSync) *SCMResumeHandler {
	return &SCMResumeHandler{sync: sync}
}

func (h *SCMResumeHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.SCMResumePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(
			fmt.Errorf("decode source control resume payload: %w", err),
			asynq.SkipRetry,
		)
	}

	return h.sync.Resume(ctx, payload.WorkspaceID, payload.IssueID)
}

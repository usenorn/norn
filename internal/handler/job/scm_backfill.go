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

type SCMBackfillHandler struct {
	sync service.SourceControlSync
}

func NewSCMBackfillHandler(sync service.SourceControlSync) *SCMBackfillHandler {
	return &SCMBackfillHandler{sync: sync}
}

func (h *SCMBackfillHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.SCMBackfillPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(
			fmt.Errorf("decode source control backfill payload: %w", err),
			asynq.SkipRetry,
		)
	}

	return h.sync.Backfill(ctx, payload.RepositoryID)
}

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

type SCMDeliveryHandler struct {
	sync service.SourceControlSync
}

func NewSCMDeliveryHandler(sync service.SourceControlSync) *SCMDeliveryHandler {
	return &SCMDeliveryHandler{sync: sync}
}

func (h *SCMDeliveryHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.SCMDeliveryPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode source control delivery payload: %w", err), asynq.SkipRetry)
	}

	if payload.DeliveryID == uuid.Nil {
		return errors.Join(errors.New("source control delivery payload is incomplete"), asynq.SkipRetry)
	}

	return h.sync.Apply(ctx, payload.DeliveryID)
}

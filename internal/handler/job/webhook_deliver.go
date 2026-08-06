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

type WebhookDeliverHandler struct {
	deliveries service.WebhookDispatch
}

func NewWebhookDeliverHandler(deliveries service.WebhookDispatch) *WebhookDeliverHandler {
	return &WebhookDeliverHandler{deliveries: deliveries}
}

func (h *WebhookDeliverHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.WebhookDeliverPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode webhook delivery payload: %w", err), asynq.SkipRetry)
	}

	if payload.DeliveryID == uuid.Nil {
		return errors.Join(errors.New("webhook delivery payload is incomplete"), asynq.SkipRetry)
	}

	return h.deliveries.Deliver(ctx, payload)
}

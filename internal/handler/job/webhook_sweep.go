package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type WebhookSweepHandler struct {
	retention service.WebhookRetention
}

func NewWebhookSweepHandler(retention service.WebhookRetention) *WebhookSweepHandler {
	return &WebhookSweepHandler{retention: retention}
}

func (h *WebhookSweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	_, err := h.retention.Sweep(ctx)

	return err
}

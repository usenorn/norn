package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type WebhookFanOutHandler struct {
	webhooks service.WebhookFanOut
	dispatch service.WebhookDispatch
}

func NewWebhookFanOutHandler(
	webhooks service.WebhookFanOut,
	dispatch service.WebhookDispatch,
) *WebhookFanOutHandler {
	return &WebhookFanOutHandler{webhooks: webhooks, dispatch: dispatch}
}

func (h *WebhookFanOutHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	if _, err := h.webhooks.FanOut(ctx); err != nil {
		return err
	}

	_, err := h.dispatch.Rescue(ctx)

	return err
}

package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type NotificationFanOutHandler struct {
	notifications service.Notifications
}

func NewNotificationFanOutHandler(notifications service.Notifications) *NotificationFanOutHandler {
	return &NotificationFanOutHandler{notifications: notifications}
}

func (h *NotificationFanOutHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	_, err := h.notifications.FanOut(ctx)

	return err
}

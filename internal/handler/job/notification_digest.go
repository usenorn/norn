package job

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type NotificationDigestHandler struct {
	notifications service.Notifications
}

func NewNotificationDigestHandler(notifications service.Notifications) *NotificationDigestHandler {
	return &NotificationDigestHandler{notifications: notifications}
}

func (h *NotificationDigestHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.notifications.Digest(ctx, time.Now().UTC())
}

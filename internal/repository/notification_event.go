package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=notification_event.go -destination=notificationevent/mock_notification_event.go -package=notificationevent -mock_names=NotificationEvent=MockNotificationEvent

type NotificationEvent interface {
	Record(ctx context.Context, event entity.NotificationEvent) error
	ClaimPending(ctx context.Context, limit int) ([]entity.NotificationEvent, error)
}

package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=notification_setting.go -destination=notificationsetting/mock_notification_setting.go -package=notificationsetting -mock_names=NotificationSetting=MockNotificationSetting

type NotificationSetting interface {
	List(ctx context.Context, workspaceID, accountID uuid.UUID) ([]entity.NotificationSettings, error)
	ListFor(
		ctx context.Context,
		workspaceID, teamID uuid.UUID,
		accountIDs []uuid.UUID,
	) ([]entity.NotificationSettings, error)
	Save(ctx context.Context, settings entity.NotificationSettings) error
	Clear(ctx context.Context, workspaceID, accountID, teamID uuid.UUID) error
}

package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=notifications.go -destination=notification/mock_notifications.go -package=notification -mock_names=Notifications=MockNotifications

type ListNotificationsInput struct {
	Filter entity.NotificationFilter
	Limit  int
	Cursor string
}

type Inbox struct {
	Notifications []entity.Notification
	Unread        int
	NextCursor    string
}

type NotificationSettingsView struct {
	Global         entity.NotificationPreferences
	Team           entity.NotificationPreferences
	TeamOverridden bool
	EmailEnabled   bool
}

type Notifications interface {
	Inbox(ctx context.Context, workspaceID uuid.UUID, input ListNotificationsInput) (Inbox, error)
	Unread(ctx context.Context, workspaceID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, workspaceID uuid.UUID, subject entity.NotificationSubject) error
	MarkAllRead(ctx context.Context, workspaceID uuid.UUID) error
	Snooze(
		ctx context.Context,
		workspaceID uuid.UUID,
		subject entity.NotificationSubject,
		until time.Time,
	) error
	Following(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueFollower, error)
	SetFollowing(
		ctx context.Context,
		workspaceID, issueID uuid.UUID,
		state entity.FollowState,
	) (entity.IssueFollower, error)
	Settings(ctx context.Context, workspaceID, teamID uuid.UUID) (NotificationSettingsView, error)
	SaveSettings(
		ctx context.Context,
		workspaceID, teamID uuid.UUID,
		preferences entity.NotificationPreferences,
	) (NotificationSettingsView, error)
	ClearSettings(ctx context.Context, workspaceID, teamID uuid.UUID) (NotificationSettingsView, error)
	FanOut(ctx context.Context) (int, error)
	Digest(ctx context.Context, now time.Time) error
}

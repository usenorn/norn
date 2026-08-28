package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=notification.go -destination=notification/mock_notification.go -package=notification -mock_names=Notification=MockNotification

type NotificationDelivery struct {
	EventID   uuid.UUID
	AccountID uuid.UUID
	Reason    entity.NotificationReason
	Channels  entity.NotificationChannels
}

type Notification interface {
	Audience(
		ctx context.Context,
		workspaceID, teamID uuid.UUID,
		accountIDs []uuid.UUID,
	) ([]uuid.UUID, error)
	Deliver(ctx context.Context, workspaceID uuid.UUID, deliveries []NotificationDelivery) error
	ListInbox(
		ctx context.Context,
		workspaceID, accountID uuid.UUID,
		page entity.NotificationPage,
	) ([]entity.Notification, error)
	Unread(ctx context.Context, workspaceID, accountID uuid.UUID) (int, error)
	MarkRead(
		ctx context.Context,
		workspaceID, accountID uuid.UUID,
		subject entity.NotificationSubject,
		at time.Time,
	) error
	MarkAllRead(ctx context.Context, workspaceID, accountID uuid.UUID, at time.Time) error
	Snooze(
		ctx context.Context,
		workspaceID, accountID uuid.UUID,
		subject entity.NotificationSubject,
		until time.Time,
	) error
	RecordView(
		ctx context.Context,
		workspaceID, accountID uuid.UUID,
		subject entity.NotificationSubject,
		at time.Time,
	) (bool, error)
	SendersAwaitingReceipt(
		ctx context.Context,
		workspaceID, accountID uuid.UUID,
		subject entity.NotificationSubject,
	) ([]uuid.UUID, error)
	Directed(
		ctx context.Context,
		workspaceID, actorID, recipientID uuid.UUID,
		limit int,
	) ([]entity.DirectedNotice, error)
	DigestRecipients(ctx context.Context, window time.Time) ([]entity.NotificationDigestClaim, error)
	DigestEntries(
		ctx context.Context,
		workspaceID, accountID uuid.UUID,
		window time.Time,
	) ([]entity.Notification, error)
	ClaimDigest(ctx context.Context, claim entity.NotificationDigestClaim) (bool, error)
	RecordDigestOutcome(ctx context.Context, claim entity.NotificationDigestClaim, failure error) error
}

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=webhook.go -destination=webhook/mock_webhook.go -package=webhook -mock_names=Webhook=MockWebhook,WebhookOutbox=MockWebhookOutbox,WebhookDelivery=MockWebhookDelivery,WebhookRetention=MockWebhookRetention

type Webhook interface {
	Create(ctx context.Context, hook entity.Webhook) (entity.Webhook, error)
	Get(ctx context.Context, workspaceID, webhookID uuid.UUID) (entity.Webhook, error)
	GetForDelivery(ctx context.Context, webhookID uuid.UUID) (entity.Webhook, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]entity.Webhook, error)
	ListSubscribed(ctx context.Context, workspaceID uuid.UUID, event entity.WebhookEvent) ([]entity.Webhook, error)
	Update(ctx context.Context, hook entity.Webhook) (entity.Webhook, error)
	RotateSecret(ctx context.Context, webhookID uuid.UUID, secret string, graceUntil time.Time) error
	RecordSuccess(ctx context.Context, webhookID uuid.UUID, at time.Time) error
	RecordFailure(ctx context.Context, webhookID uuid.UUID) (int, error)
	Disable(ctx context.Context, webhookID uuid.UUID, reason entity.WebhookDisableReason, at time.Time) (bool, error)
	Enable(ctx context.Context, webhookID uuid.UUID) error
	Delete(ctx context.Context, workspaceID, webhookID uuid.UUID) error
}

type WebhookOutbox interface {
	Record(ctx context.Context, entry entity.WebhookOutboxEntry) error
	ClaimPending(ctx context.Context, limit int) ([]entity.WebhookOutboxEntry, error)
}

type WebhookDelivery interface {
	Queue(ctx context.Context, deliveries []entity.WebhookDelivery) error
	Get(ctx context.Context, webhookID, deliveryID uuid.UUID) (entity.WebhookDelivery, error)
	List(ctx context.Context, filter entity.WebhookDeliveryFilter) ([]entity.WebhookDelivery, error)
	Attempts(ctx context.Context, deliveryID uuid.UUID) ([]entity.WebhookAttempt, error)
	ClaimAttempt(ctx context.Context, deliveryID uuid.UUID, attempt int) (entity.WebhookDelivery, error)
	RecordAttempt(ctx context.Context, attempt entity.WebhookAttempt) error
	Reschedule(ctx context.Context, deliveryID uuid.UUID, at time.Time) error
	Settle(ctx context.Context, deliveryID uuid.UUID, state entity.WebhookDeliveryState, at time.Time) error
	ListDue(ctx context.Context, before time.Time, limit int) ([]entity.WebhookDelivery, error)
}

type WebhookRetention interface {
	DropDeliveriesBefore(ctx context.Context, cutoff time.Time, batch int) (int, error)
	DropOutboxBefore(ctx context.Context, cutoff time.Time, batch int) (int, error)
}

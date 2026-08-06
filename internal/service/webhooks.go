package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=webhooks.go -destination=webhook/mock_webhooks.go -package=webhook -mock_names=WebhookEmitter=MockWebhookEmitter,Webhooks=MockWebhooks,WebhookDeliveries=MockWebhookDeliveries,WebhookFanOut=MockWebhookFanOut,WebhookDispatch=MockWebhookDispatch,WebhookRetention=MockWebhookRetention

type WebhookEmitter interface {
	Emit(ctx context.Context, entry entity.WebhookOutboxEntry) error
}

type RegisterWebhookInput struct {
	WorkspaceID uuid.UUID
	Name        string
	URL         string
	Events      []entity.WebhookEvent
}

type UpdateWebhookInput struct {
	Name    *string
	URL     *string
	Events  []entity.WebhookEvent
	Enabled *bool
}

type Webhooks interface {
	List(ctx context.Context, workspaceID uuid.UUID) ([]entity.Webhook, error)
	Get(ctx context.Context, workspaceID, webhookID uuid.UUID) (entity.Webhook, error)
	Register(ctx context.Context, input RegisterWebhookInput) (entity.Webhook, error)
	Update(ctx context.Context, workspaceID, webhookID uuid.UUID, input UpdateWebhookInput) (entity.Webhook, error)
	RotateSecret(ctx context.Context, workspaceID, webhookID uuid.UUID) (entity.Webhook, error)
	Remove(ctx context.Context, workspaceID, webhookID uuid.UUID) error
}

type ListWebhookDeliveriesInput struct {
	Events []entity.WebhookEvent
	States []entity.WebhookDeliveryState
	Cursor string
	Limit  int
}

type WebhookDeliveryPage struct {
	Deliveries []entity.WebhookDelivery
	NextCursor string
}

type WebhookDeliveries interface {
	List(ctx context.Context, workspaceID, webhookID uuid.UUID, input ListWebhookDeliveriesInput) (WebhookDeliveryPage, error)
	Get(ctx context.Context, workspaceID, webhookID, deliveryID uuid.UUID) (entity.WebhookDelivery, error)
	Test(ctx context.Context, workspaceID, webhookID uuid.UUID) (entity.WebhookDelivery, error)
	Replay(ctx context.Context, workspaceID, webhookID, deliveryID uuid.UUID) (entity.WebhookDelivery, error)
}

type WebhookFanOut interface {
	FanOut(ctx context.Context) (int, error)
}

type WebhookDispatch interface {
	Deliver(ctx context.Context, payload entity.WebhookDeliverPayload) error
	Rescue(ctx context.Context) (int, error)
}

type WebhookRetention interface {
	Sweep(ctx context.Context) (int, error)
}

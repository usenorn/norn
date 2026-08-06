package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=webhook_sender.go -destination=webhooksender/mock_webhook_sender.go -package=webhooksender -mock_names=WebhookSender=MockWebhookSender

type WebhookSender interface {
	Send(ctx context.Context, request entity.WebhookRequest) entity.WebhookResponse
	Check(ctx context.Context, url string) error
}

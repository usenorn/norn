package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=preview_gateways.go -destination=previewgateway/mock_preview_gateways.go -package=previewgateway -mock_names=PreviewGateways=MockPreviewGateways

type PreviewGatewayAccess struct {
	Token     string
	ExpiresAt time.Time
}

type PreviewGateways interface {
	Adopt(ctx context.Context, name, secret string) (entity.PreviewGateway, error)
	Enrol(ctx context.Context, name string) (entity.PreviewGateway, string, error)
	Exchange(ctx context.Context, secret string) (PreviewGatewayAccess, error)
	Authenticate(ctx context.Context, token string) (entity.PreviewGateway, error)
	List(ctx context.Context) ([]entity.PreviewGateway, error)
	Revoke(ctx context.Context, gatewayID uuid.UUID) (entity.PreviewGateway, error)
}

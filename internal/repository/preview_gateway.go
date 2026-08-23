package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=preview_gateway.go -destination=previewgateway/mock_preview_gateway.go -package=previewgateway -mock_names=PreviewGateway=MockPreviewGateway

type PreviewGateway interface {
	Create(
		ctx context.Context,
		gateway entity.PreviewGateway,
		secretHash []byte,
	) (entity.PreviewGateway, error)
	ByCredential(ctx context.Context, secretHash []byte) (entity.PreviewGateway, error)
	List(ctx context.Context) ([]entity.PreviewGateway, error)
	Revoke(ctx context.Context, gatewayID uuid.UUID) (entity.PreviewGateway, error)
	Seen(ctx context.Context, gatewayID uuid.UUID, at time.Time) error
}

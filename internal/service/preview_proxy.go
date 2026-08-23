package service

import (
	"context"
	"net"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=preview_proxy.go -destination=previewproxy/mock_preview_proxy.go -package=previewproxy -mock_names=PreviewProxy=MockPreviewProxy

type PreviewProxy interface {
	Run(ctx context.Context)
	Ready() bool
	Route(ctx context.Context, ask entity.PreviewAsk) (entity.PreviewReply, error)
	Session(ctx context.Context, ticket string) (entity.PreviewGrantReply, error)
	Redeem(ctx context.Context, host, share, passcode string) (entity.PreviewGrantReply, error)
	Accept(ctx context.Context, ticket string) (entity.TunnelClaim, error)
	Hold(ctx context.Context, runnerID uuid.UUID, socket net.Conn) error
	Dial(ctx context.Context, reply entity.PreviewReply) (net.Conn, error)
}

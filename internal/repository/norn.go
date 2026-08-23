package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=norn.go -destination=nornapi/mock_norn.go -package=nornapi -mock_names=Norn=MockNorn

type Norn interface {
	Exchange(ctx context.Context, secret string) (entity.PreviewGatewayToken, error)
	Introspect(ctx context.Context, token string, ask entity.PreviewAsk) (entity.PreviewReply, error)
	Session(ctx context.Context, token, ticket string) (entity.PreviewGrantReply, error)
	Redeem(ctx context.Context, token, host, share, passcode string) (entity.PreviewGrantReply, error)
	Tunnel(ctx context.Context, token, ticket string) (entity.TunnelClaim, error)
}

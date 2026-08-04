package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=oidcprovider.go -destination=oidcprovider/mock_oidcprovider.go -package=oidcprovider -mock_names=OIDCProvider=MockOIDCProvider

type OIDCProvider interface {
	Discover(ctx context.Context, issuer string) (entity.OIDCEndpoints, error)
	AuthCodeURL(ctx context.Context, connection entity.OIDCConnection, attempt entity.OIDCAuthorization) string
	Exchange(ctx context.Context, connection entity.OIDCConnection, attempt entity.OIDCRedemption) (entity.OIDCClaims, error)
}

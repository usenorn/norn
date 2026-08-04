package oidcprovider

import (
	"context"
	"strings"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/oidcprovider"
	"github.com/usenorn/norn/internal/repository"
)

const callbackPath = "/v1/sso/oidc/callback"

type providerRepository struct {
	client *oidcprovider.Client
	app    config.App
}

func New(client *oidcprovider.Client, app config.App) repository.OIDCProvider {
	return &providerRepository{client: client, app: app}
}

func (r *providerRepository) redirectURI() string {
	return strings.TrimRight(r.app.BaseURL, "/") + callbackPath
}

func (r *providerRepository) Discover(
	ctx context.Context,
	issuer string,
) (entity.OIDCEndpoints, error) {
	return r.client.Discover(ctx, issuer)
}

func (r *providerRepository) AuthCodeURL(
	ctx context.Context,
	connection entity.OIDCConnection,
	attempt entity.OIDCAuthorization,
) string {
	return r.client.
		For(ctx, connection, r.redirectURI()).
		AuthCodeURL(attempt.State, attempt.Nonce, attempt.Verifier)
}

func (r *providerRepository) Exchange(
	ctx context.Context,
	connection entity.OIDCConnection,
	attempt entity.OIDCRedemption,
) (entity.OIDCClaims, error) {
	return r.client.
		For(ctx, connection, r.redirectURI()).
		Exchange(ctx, attempt.Code, attempt.Verifier, attempt.Nonce)
}

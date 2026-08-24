package scm

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type credentials struct {
	connections repository.SCMConnection
	apps        repository.SCMApp
	forges      service.Forges
	cache       *forge.Credentials
}

func newCredentials(
	connections repository.SCMConnection,
	apps repository.SCMApp,
	forges service.Forges,
	cache *forge.Credentials,
) *credentials {
	return &credentials{connections: connections, apps: apps, forges: forges, cache: cache}
}

func (c *credentials) application(
	ctx context.Context,
	provider entity.SCMProvider,
	baseURL string,
) (entity.SCMApp, error) {
	endpoint := strings.TrimSpace(baseURL)

	if endpoint == "" {
		forge, err := c.forges.Lookup(provider)
		if err != nil {
			return entity.SCMApp{}, err
		}

		endpoint = forge.Endpoint()
	}

	return c.apps.Get(ctx, provider, endpoint)
}

func (c *credentials) identify(
	ctx context.Context,
	target entity.SCMTarget,
	kind entity.SCMAuthKind,
	accountLogin string,
) (string, error) {
	forge, err := c.forges.Lookup(target.Provider)
	if err != nil {
		return "", err
	}

	if kind != entity.SCMAuthApp {
		return forge.Identity(ctx, target)
	}

	registered, err := c.application(ctx, target.Provider, target.BaseURL)
	if err != nil {
		return "", err
	}

	forgeApp, err := c.forges.App(target.Provider)
	if err != nil {
		return "", err
	}

	if _, err := forgeApp.InstallationRepositories(ctx, registered, target.Token); err != nil {
		return "", err
	}

	if registered.Slug != "" {
		return registered.Slug + entity.SCMAppLoginSuffix, nil
	}

	return accountLogin, nil
}

func (c *credentials) token(
	ctx context.Context,
	connection entity.SCMConnection,
) (string, error) {
	if !connection.UsesApp() {
		return c.connections.Token(ctx, connection.ID)
	}

	if held, found := c.cache.Get(connection.ID, time.Now()); found {
		return held, nil
	}

	return c.mint(ctx, connection)
}

// refresh spends a new installation token even when a usable one is held. An installation token
// carries the repositories the installation reached when it was minted, so a repository granted
// afterwards stays invisible to it for the rest of its hour — which is exactly the moment
// somebody comes here to connect the repository they have just granted.
func (c *credentials) refresh(
	ctx context.Context,
	connection entity.SCMConnection,
) (string, error) {
	if !connection.UsesApp() {
		return c.connections.Token(ctx, connection.ID)
	}

	return c.mint(ctx, connection)
}

func (c *credentials) mint(
	ctx context.Context,
	connection entity.SCMConnection,
) (string, error) {
	if connection.AppID == uuid.Nil || connection.InstallationID == "" {
		return "", entity.ErrSCMInstallationNotFound
	}

	app, err := c.apps.Secrets(ctx, connection.AppID)
	if err != nil {
		return "", err
	}

	forgeApp, err := c.forges.App(connection.Provider)
	if err != nil {
		return "", err
	}

	minted, err := forgeApp.MintInstallationToken(ctx, app, connection.InstallationID)
	if err != nil {
		return "", err
	}

	c.cache.Put(connection.ID, minted)

	return minted.Token, nil
}

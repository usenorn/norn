package scm

import (
	"context"
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

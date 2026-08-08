package scm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type appRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func NewSCMApp(db *postgres.Client, sealer *crypter.Crypter) repository.SCMApp {
	return &appRepository{db: db, crypter: sealer}
}

const appColumns = `
    id, provider, base_url, slug, external_app_id, client_id, created_at, updated_at`

func scanApp(row interface{ Scan(...any) error }) (entity.SCMApp, error) {
	var app entity.SCMApp

	err := row.Scan(
		&app.ID,
		&app.Provider,
		&app.BaseURL,
		&app.Slug,
		&app.ExternalAppID,
		&app.ClientID,
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		return entity.SCMApp{}, err
	}

	return app, nil
}

const upsertAppQuery = `
INSERT INTO scm_apps (
    id, provider, base_url, slug, external_app_id, client_id, client_secret_sealed,
    private_key_sealed, webhook_secret_sealed
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (provider, base_url) DO UPDATE
SET slug = EXCLUDED.slug,
    external_app_id = EXCLUDED.external_app_id,
    client_id = EXCLUDED.client_id,
    client_secret_sealed = EXCLUDED.client_secret_sealed,
    private_key_sealed = EXCLUDED.private_key_sealed,
    webhook_secret_sealed = EXCLUDED.webhook_secret_sealed,
    updated_at = now()
RETURNING` + appColumns

func (r *appRepository) Upsert(
	ctx context.Context,
	input repository.SCMAppInput,
) (entity.SCMApp, error) {
	app := input.App

	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}

	key, err := seal(r.crypter, input.PrivateKey)
	if err != nil {
		return entity.SCMApp{}, err
	}

	secret, err := seal(r.crypter, input.WebhookSecret)
	if err != nil {
		return entity.SCMApp{}, err
	}

	var client []byte

	if input.ClientSecret != "" {
		if client, err = seal(r.crypter, input.ClientSecret); err != nil {
			return entity.SCMApp{}, err
		}
	}

	stored, err := scanApp(r.db.Querier(ctx).QueryRowContext(
		ctx,
		upsertAppQuery,
		app.ID,
		app.Provider,
		app.BaseURL,
		app.Slug,
		app.ExternalAppID,
		app.ClientID,
		client,
		key,
		secret,
	))
	if err != nil {
		return entity.SCMApp{}, fmt.Errorf("register a source control application: %w", err)
	}

	return stored, nil
}

const getAppQuery = `
SELECT` + appColumns + `
FROM scm_apps
WHERE provider = $1 AND base_url = $2`

func (r *appRepository) Get(
	ctx context.Context,
	provider entity.SCMProvider,
	baseURL string,
) (entity.SCMApp, error) {
	app, err := scanApp(r.db.Querier(ctx).QueryRowContext(ctx, getAppQuery, provider, baseURL))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMApp{}, entity.ErrSCMAppNotFound
	}

	if err != nil {
		return entity.SCMApp{}, fmt.Errorf("read a source control application: %w", err)
	}

	return app, nil
}

const getAppByIDQuery = `
SELECT` + appColumns + `
FROM scm_apps
WHERE id = $1`

func (r *appRepository) GetByID(ctx context.Context, appID uuid.UUID) (entity.SCMApp, error) {
	app, err := scanApp(r.db.Querier(ctx).QueryRowContext(ctx, getAppByIDQuery, appID))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMApp{}, entity.ErrSCMAppNotFound
	}

	if err != nil {
		return entity.SCMApp{}, fmt.Errorf("read a source control application: %w", err)
	}

	return app, nil
}

const appSecretsQuery = `
SELECT` + appColumns + `, private_key_sealed, webhook_secret_sealed, client_secret_sealed
FROM scm_apps
WHERE id = $1`

func (r *appRepository) Secrets(ctx context.Context, appID uuid.UUID) (entity.SCMApp, error) {
	var (
		app                        entity.SCMApp
		key, webhook, clientSecret []byte
	)

	err := r.db.Querier(ctx).QueryRowContext(ctx, appSecretsQuery, appID).Scan(
		&app.ID,
		&app.Provider,
		&app.BaseURL,
		&app.Slug,
		&app.ExternalAppID,
		&app.ClientID,
		&app.CreatedAt,
		&app.UpdatedAt,
		&key,
		&webhook,
		&clientSecret,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMApp{}, entity.ErrSCMAppNotFound
	}

	if err != nil {
		return entity.SCMApp{}, fmt.Errorf("read source control application secrets: %w", err)
	}

	if app.PrivateKey, err = open(r.crypter, key); err != nil {
		return entity.SCMApp{}, err
	}

	if app.WebhookSecret, err = open(r.crypter, webhook); err != nil {
		return entity.SCMApp{}, err
	}

	if app.ClientSecret, err = open(r.crypter, clientSecret); err != nil {
		return entity.SCMApp{}, err
	}

	return app, nil
}

const listAppsQuery = `
SELECT` + appColumns + `
FROM scm_apps
ORDER BY provider, base_url`

func (r *appRepository) List(ctx context.Context) ([]entity.SCMApp, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, listAppsQuery)
	if err != nil {
		return nil, fmt.Errorf("list source control applications: %w", err)
	}

	defer func() { _ = rows.Close() }()

	apps := make([]entity.SCMApp, 0)

	for rows.Next() {
		app, err := scanApp(rows)
		if err != nil {
			return nil, fmt.Errorf("read a source control application: %w", err)
		}

		apps = append(apps, app)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list source control applications: %w", err)
	}

	return apps, nil
}

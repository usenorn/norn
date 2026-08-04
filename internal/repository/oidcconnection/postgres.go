package oidcconnection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const connectionColumns = `workspace_id, issuer, discovered, authorization_endpoint, token_endpoint,
       jwks_uri, userinfo_endpoint, client_id, client_secret_sealed, scopes,
       groups_claim, provisioning, verified_at, created_at, updated_at`

const connectionByWorkspaceQuery = `
SELECT ` + connectionColumns + `
FROM workspace_oidc_connections
WHERE workspace_id = $1`

const upsertConnectionQuery = `
INSERT INTO workspace_oidc_connections (
    workspace_id, issuer, discovered, authorization_endpoint, token_endpoint,
    jwks_uri, userinfo_endpoint, client_id, client_secret_sealed, scopes,
    groups_claim, provisioning, verified_at, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULL, $13, $13)
ON CONFLICT (workspace_id) DO UPDATE SET
    issuer                 = excluded.issuer,
    discovered             = excluded.discovered,
    authorization_endpoint = excluded.authorization_endpoint,
    token_endpoint         = excluded.token_endpoint,
    jwks_uri               = excluded.jwks_uri,
    userinfo_endpoint      = excluded.userinfo_endpoint,
    client_id              = excluded.client_id,
    client_secret_sealed   = excluded.client_secret_sealed,
    scopes                 = excluded.scopes,
    groups_claim           = excluded.groups_claim,
    provisioning           = excluded.provisioning,
    verified_at            = NULL,
    updated_at             = excluded.updated_at
RETURNING ` + connectionColumns

const deleteConnectionQuery = `DELETE FROM workspace_oidc_connections WHERE workspace_id = $1`

const markVerifiedQuery = `
UPDATE workspace_oidc_connections
SET verified_at = $2, updated_at = $2
WHERE workspace_id = $1`

type connectionRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func New(db *postgres.Client, sealer *crypter.Crypter) repository.OIDCConnection {
	return &connectionRepository{db: db, crypter: sealer}
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *connectionRepository) scan(row scanner) (entity.OIDCConnection, error) {
	var (
		connection entity.OIDCConnection
		workspace  string
		sealed     []byte
		scopes     types.StringArray
		verified   sql.NullTime
	)

	if err := row.Scan(
		&workspace,
		&connection.Endpoints.Issuer,
		&connection.Discovered,
		&connection.Endpoints.AuthorizationEndpoint,
		&connection.Endpoints.TokenEndpoint,
		&connection.Endpoints.JWKSURI,
		&connection.Endpoints.UserinfoEndpoint,
		&connection.ClientID,
		&sealed,
		&scopes,
		&connection.GroupsClaim,
		&connection.Provisioning,
		&verified,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	); err != nil {
		return entity.OIDCConnection{}, err
	}

	parsed, err := uuid.Parse(workspace)
	if err != nil {
		return entity.OIDCConnection{}, fmt.Errorf("parse oidc connection workspace id: %w", err)
	}

	connection.WorkspaceID = parsed
	connection.Scopes = scopes

	secret, err := r.crypter.Open(sealed)
	if err != nil {
		return entity.OIDCConnection{}, fmt.Errorf("open oidc client secret: %w", err)
	}

	connection.ClientSecret = string(secret)

	if verified.Valid {
		at := verified.Time
		connection.VerifiedAt = &at
	}

	return connection, nil
}

func (r *connectionRepository) Get(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.OIDCConnection, error) {
	connection, err := r.scan(r.db.Querier(ctx).QueryRowContext(
		ctx,
		connectionByWorkspaceQuery,
		workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.OIDCConnection{}, entity.ErrOIDCConnectionNotFound
		}

		return entity.OIDCConnection{}, fmt.Errorf("find oidc connection: %w", err)
	}

	return connection, nil
}

func (r *connectionRepository) Save(
	ctx context.Context,
	connection entity.OIDCConnection,
) (entity.OIDCConnection, error) {
	sealed, err := r.crypter.Seal([]byte(connection.ClientSecret))
	if err != nil {
		return entity.OIDCConnection{}, err
	}

	saved, err := r.scan(r.db.Querier(ctx).QueryRowContext(
		ctx,
		upsertConnectionQuery,
		connection.WorkspaceID.String(),
		connection.Endpoints.Issuer,
		connection.Discovered,
		connection.Endpoints.AuthorizationEndpoint,
		connection.Endpoints.TokenEndpoint,
		connection.Endpoints.JWKSURI,
		connection.Endpoints.UserinfoEndpoint,
		connection.ClientID,
		sealed,
		types.StringArray(entity.NormalizeOIDCScopes(connection.Scopes)),
		connection.GroupsClaim,
		connection.Provisioning,
		time.Now().UTC(),
	))
	if err != nil {
		return entity.OIDCConnection{}, fmt.Errorf("save oidc connection: %w", err)
	}

	return saved, nil
}

func (r *connectionRepository) Delete(ctx context.Context, workspaceID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteConnectionQuery, workspaceID.String())
	if err != nil {
		return fmt.Errorf("delete oidc connection: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted oidc connection rows: %w", err)
	}

	if removed == 0 {
		return entity.ErrOIDCConnectionNotFound
	}

	return nil
}

func (r *connectionRepository) MarkVerified(
	ctx context.Context,
	workspaceID uuid.UUID,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		markVerifiedQuery,
		workspaceID.String(),
		at.UTC(),
	)
	if err != nil {
		return fmt.Errorf("mark oidc connection verified: %w", err)
	}

	marked, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read verified oidc connection rows: %w", err)
	}

	if marked == 0 {
		return entity.ErrOIDCConnectionNotFound
	}

	return nil
}

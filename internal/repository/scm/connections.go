package scm

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

type connectionRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func NewSCMConnection(db *postgres.Client, sealer *crypter.Crypter) repository.SCMConnection {
	return &connectionRepository{db: db, crypter: sealer}
}

func seal(sealer *crypter.Crypter, secret string) ([]byte, error) {
	sealed, err := sealer.Seal([]byte(secret))
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return nil, entity.ErrSCMEncryptionKeyMissing
		}

		return nil, fmt.Errorf("seal source control credential: %w", err)
	}

	return sealed, nil
}

func credential(
	sealer *crypter.Crypter,
	kind entity.SCMAuthKind,
	token string,
) ([]byte, error) {
	if kind == entity.SCMAuthApp {
		return nil, nil
	}

	return seal(sealer, token)
}

func appOrNil(appID uuid.UUID) *uuid.UUID {
	if appID == uuid.Nil {
		return nil
	}

	return &appID
}

func open(sealer *crypter.Crypter, sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}

	secret, err := sealer.Open(sealed)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return "", entity.ErrSCMEncryptionKeyMissing
		}

		return "", fmt.Errorf("open stored source control credential: %w", err)
	}

	return string(secret), nil
}

const connectionColumns = `
    id, workspace_id, provider, base_url, label,
    token_hint, coalesce(octet_length(token_sealed), 0) > 0, identity_login,
    integration_account_id, owner_account_id, owner_actor_kind, owner_auth_method,
    status, broken_reason, broken_detail, broken_at, verified_at,
    auth_kind, coalesce(app_id, '00000000-0000-0000-0000-000000000000'::uuid),
    installation_id, account_login,
    allow_private_address, ca_certificate, capabilities, created_at, updated_at`

func scanConnection(row interface{ Scan(...any) error }) (entity.SCMConnection, error) {
	var (
		connection   entity.SCMConnection
		capabilities types.StringArray
	)

	err := row.Scan(
		&connection.ID,
		&connection.WorkspaceID,
		&connection.Provider,
		&connection.BaseURL,
		&connection.Label,
		&connection.TokenHint,
		&connection.TokenSet,
		&connection.IdentityLogin,
		&connection.IntegrationAccountID,
		&connection.OwnerAccountID,
		&connection.OwnerActorKind,
		&connection.OwnerAuthMethod,
		&connection.Status,
		&connection.BrokenReason,
		&connection.BrokenDetail,
		&connection.BrokenAt,
		&connection.VerifiedAt,
		&connection.AuthKind,
		&connection.AppID,
		&connection.InstallationID,
		&connection.AccountLogin,
		&connection.Trust.AllowPrivateAddress,
		&connection.Trust.CACertificate,
		&capabilities,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	connection.Capabilities = entity.SCMCapabilitiesFrom(capabilities)
	connection.IntegrationName = entity.IntegrationAccountName(
		connection.Provider,
		connection.DisplayName(),
	)

	return connection, nil
}

const insertConnectionQuery = `
INSERT INTO workspace_scm_connections (
    id, workspace_id, provider, base_url, label, token_sealed, token_hint, identity_login,
    integration_account_id, owner_account_id, owner_actor_kind, owner_auth_method,
    auth_kind, app_id, installation_id, account_login,
    allow_private_address, ca_certificate, capabilities
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING` + connectionColumns

func (r *connectionRepository) Create(
	ctx context.Context,
	input repository.SCMConnectionInput,
) (entity.SCMConnection, error) {
	connection := input.Connection

	if connection.ID == uuid.Nil {
		connection.ID = uuid.New()
	}

	if connection.AuthKind == "" {
		connection.AuthKind = entity.SCMAuthToken
	}

	token, err := credential(r.crypter, connection.AuthKind, input.Token)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	created, err := scanConnection(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertConnectionQuery,
		connection.ID,
		connection.WorkspaceID,
		connection.Provider,
		connection.BaseURL,
		connection.Label,
		token,
		connection.TokenHint,
		connection.IdentityLogin,
		connection.IntegrationAccountID,
		connection.OwnerAccountID,
		connection.OwnerActorKind,
		connection.OwnerAuthMethod,
		connection.AuthKind,
		appOrNil(connection.AppID),
		connection.InstallationID,
		connection.AccountLogin,
		connection.Trust.AllowPrivateAddress,
		connection.Trust.CACertificate,
		types.StringArray(connection.Capabilities.Strings()),
	))
	if err != nil {
		if violates(err, connectionEndpointUniqueIndex) {
			return entity.SCMConnection{}, entity.ErrSCMConnectionExists
		}

		return entity.SCMConnection{}, fmt.Errorf("create source control connection: %w", err)
	}

	return created, nil
}

const getConnectionQuery = `
SELECT` + connectionColumns + `
FROM workspace_scm_connections
WHERE workspace_id = $1 AND id = $2`

func (r *connectionRepository) GetByID(
	ctx context.Context,
	workspaceID, connectionID uuid.UUID,
) (entity.SCMConnection, error) {
	connection, err := scanConnection(
		r.db.Querier(ctx).QueryRowContext(ctx, getConnectionQuery, workspaceID, connectionID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMConnection{}, entity.ErrSCMConnectionNotFound
	}

	if err != nil {
		return entity.SCMConnection{}, fmt.Errorf("read source control connection: %w", err)
	}

	return connection, nil
}

const getConnectionForDeliveryQuery = `
SELECT` + connectionColumns + `
FROM workspace_scm_connections
WHERE id = $1`

func (r *connectionRepository) GetForDelivery(
	ctx context.Context,
	connectionID uuid.UUID,
) (entity.SCMConnection, error) {
	connection, err := scanConnection(
		r.db.Querier(ctx).QueryRowContext(ctx, getConnectionForDeliveryQuery, connectionID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMConnection{}, entity.ErrSCMConnectionNotFound
	}

	if err != nil {
		return entity.SCMConnection{}, fmt.Errorf("read source control connection: %w", err)
	}

	return connection, nil
}

const listConnectionsQuery = `
SELECT` + connectionColumns + `
FROM workspace_scm_connections
WHERE workspace_id = $1
ORDER BY created_at DESC, id`

func (r *connectionRepository) ListByWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.SCMConnection, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, listConnectionsQuery, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list source control connections: %w", err)
	}

	defer func() { _ = rows.Close() }()

	connections := make([]entity.SCMConnection, 0)

	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			return nil, fmt.Errorf("read source control connection: %w", err)
		}

		connections = append(connections, connection)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list source control connections: %w", err)
	}

	return connections, nil
}

const tokenQuery = `
SELECT token_sealed
FROM workspace_scm_connections
WHERE id = $1`

func (r *connectionRepository) Token(ctx context.Context, connectionID uuid.UUID) (string, error) {
	var sealed []byte

	err := r.db.Querier(ctx).QueryRowContext(ctx, tokenQuery, connectionID).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return "", entity.ErrSCMConnectionNotFound
	}

	if err != nil {
		return "", fmt.Errorf("read source control credentials: %w", err)
	}

	if len(sealed) == 0 {
		return "", entity.ErrSCMInstallationNotFound
	}

	return open(r.crypter, sealed)
}

const replaceTokenQuery = `
UPDATE workspace_scm_connections
SET token_sealed = $2,
    token_hint = $3,
    identity_login = $4,
    status = 'connected',
    broken_reason = '',
    broken_detail = '',
    broken_at = NULL,
    verified_at = $5,
    updated_at = now()
WHERE id = $1`

func (r *connectionRepository) ReplaceToken(
	ctx context.Context,
	connectionID uuid.UUID,
	token, hint, login string,
	at time.Time,
) error {
	sealed, err := seal(r.crypter, token)
	if err != nil {
		return err
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx, replaceTokenQuery, connectionID, sealed, hint, login, at,
	)
	if err != nil {
		return fmt.Errorf("replace source control token: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const updateLabelQuery = `
UPDATE workspace_scm_connections
SET label = $2, updated_at = now()
WHERE id = $1
RETURNING` + connectionColumns

func (r *connectionRepository) UpdateLabel(
	ctx context.Context,
	connectionID uuid.UUID,
	label string,
) (entity.SCMConnection, error) {
	connection, err := scanConnection(
		r.db.Querier(ctx).QueryRowContext(ctx, updateLabelQuery, connectionID, label),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMConnection{}, entity.ErrSCMConnectionNotFound
	}

	if err != nil {
		return entity.SCMConnection{}, fmt.Errorf("update source control connection: %w", err)
	}

	return connection, nil
}

const markVerifiedQuery = `
UPDATE workspace_scm_connections
SET identity_login = $2,
    capabilities = $4,
    status = 'connected',
    broken_reason = '',
    broken_detail = '',
    broken_at = NULL,
    verified_at = $3,
    updated_at = now()
WHERE id = $1`

func (r *connectionRepository) MarkVerified(
	ctx context.Context,
	connectionID uuid.UUID,
	login string,
	capabilities entity.SCMCapabilitySet,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		markVerifiedQuery,
		connectionID,
		login,
		at,
		types.StringArray(capabilities.Strings()),
	)
	if err != nil {
		return fmt.Errorf("mark source control connection verified: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const markBrokenQuery = `
UPDATE workspace_scm_connections
SET status = 'broken',
    broken_reason = $2,
    broken_detail = $3,
    broken_at = $4,
    updated_at = now()
WHERE id = $1`

func (r *connectionRepository) MarkBroken(
	ctx context.Context,
	connectionID uuid.UUID,
	reason entity.SCMBrokenReason,
	detail string,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, markBrokenQuery, connectionID, reason, detail, at,
	)
	if err != nil {
		return fmt.Errorf("mark source control connection broken: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const deleteConnectionQuery = `
DELETE FROM workspace_scm_connections
WHERE id = $1`

func (r *connectionRepository) Delete(ctx context.Context, connectionID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteConnectionQuery, connectionID)
	if err != nil {
		return fmt.Errorf("delete source control connection: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

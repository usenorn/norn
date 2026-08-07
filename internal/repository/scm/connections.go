package scm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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

func (r *connectionRepository) seal(secret string) ([]byte, error) {
	sealed, err := r.crypter.Seal([]byte(secret))
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return nil, entity.ErrSCMEncryptionKeyMissing
		}

		return nil, fmt.Errorf("seal source control credential: %w", err)
	}

	return sealed, nil
}

func (r *connectionRepository) open(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}

	secret, err := r.crypter.Open(sealed)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return "", entity.ErrSCMEncryptionKeyMissing
		}

		return "", fmt.Errorf("open stored source control credential: %w", err)
	}

	return string(secret), nil
}

// connectionColumns never selects a sealed column. The plaintext leaves this repository
// through Credentials alone, which is called on the way into a forge call and nowhere the
// dashboard can reach; everything else reads whether a token is set, not what it is.
const connectionColumns = `
    id, workspace_id, coalesce(team_id, '00000000-0000-0000-0000-000000000000'::uuid),
    provider, base_url, repository, external_repository_id,
    token_hint, token_sealed IS NOT NULL, identity_login, external_hook_id,
    integration_account_id, owner_account_id, owner_actor_kind, owner_auth_method,
    mirror_label, status, broken_reason, broken_detail, broken_at, verified_at,
    last_seen_at, reconcile_cursor, reconciled_at, reconcile_after, created_at, updated_at`

func scanConnection(row interface{ Scan(...any) error }) (entity.SCMConnection, error) {
	var (
		connection entity.SCMConnection
		teamID     uuid.UUID
	)

	err := row.Scan(
		&connection.ID,
		&connection.WorkspaceID,
		&teamID,
		&connection.Provider,
		&connection.BaseURL,
		&connection.Repository,
		&connection.ExternalRepositoryID,
		&connection.TokenHint,
		&connection.TokenSet,
		&connection.IdentityLogin,
		&connection.ExternalHookID,
		&connection.IntegrationAccountID,
		&connection.OwnerAccountID,
		&connection.OwnerActorKind,
		&connection.OwnerAuthMethod,
		&connection.MirrorLabel,
		&connection.Status,
		&connection.BrokenReason,
		&connection.BrokenDetail,
		&connection.BrokenAt,
		&connection.VerifiedAt,
		&connection.LastSeenAt,
		&connection.ReconcileCursor,
		&connection.ReconciledAt,
		&connection.ReconcileAfter,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	connection.TeamID = teamID
	connection.IntegrationName = entity.IntegrationAccountName(connection.Provider, connection.Repository)

	return connection, nil
}

const insertConnectionQuery = `
INSERT INTO workspace_scm_connections (
    id, workspace_id, team_id, provider, base_url, repository, external_repository_id,
    token_sealed, token_hint, identity_login, webhook_secret_sealed,
    integration_account_id, owner_account_id, owner_actor_kind, owner_auth_method, mirror_label
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING` + connectionColumns

func (r *connectionRepository) Create(
	ctx context.Context,
	input repository.SCMConnectionInput,
) (entity.SCMConnection, error) {
	connection := input.Connection

	if connection.ID == uuid.Nil {
		connection.ID = uuid.New()
	}

	token, err := r.seal(input.Token)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	secret, err := r.seal(input.WebhookSecret)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	created, err := scanConnection(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertConnectionQuery,
		connection.ID,
		connection.WorkspaceID,
		teamOrNil(connection.TeamID),
		connection.Provider,
		connection.BaseURL,
		connection.Repository,
		connection.ExternalRepositoryID,
		token,
		connection.TokenHint,
		connection.IdentityLogin,
		secret,
		connection.IntegrationAccountID,
		connection.OwnerAccountID,
		connection.OwnerActorKind,
		connection.OwnerAuthMethod,
		connection.MirrorLabel,
	))
	if err != nil {
		if violates(err, repositoryUniqueIndex) {
			return entity.SCMConnection{}, entity.ErrSCMConnectionExists
		}

		return entity.SCMConnection{}, fmt.Errorf("create source control connection: %w", err)
	}

	return created, nil
}

func teamOrNil(teamID uuid.UUID) any {
	if teamID == uuid.Nil {
		return nil
	}

	return teamID
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

// GetForDelivery reads a connection by its id alone, because an inbound delivery arrives
// with nothing but the address it was sent to. Everything it authorises afterwards is
// decided from the connection's own workspace, never from anything the caller said.
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
			return nil, fmt.Errorf("scan source control connection: %w", err)
		}

		connections = append(connections, connection)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read source control connections: %w", err)
	}

	return connections, nil
}

const credentialsQuery = `
SELECT token_sealed, webhook_secret_sealed
FROM workspace_scm_connections
WHERE id = $1`

func (r *connectionRepository) Credentials(
	ctx context.Context,
	connectionID uuid.UUID,
) (repository.SCMCredentials, error) {
	var token, secret []byte

	err := r.db.Querier(ctx).
		QueryRowContext(ctx, credentialsQuery, connectionID).
		Scan(&token, &secret)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.SCMCredentials{}, entity.ErrSCMConnectionNotFound
	}

	if err != nil {
		return repository.SCMCredentials{}, fmt.Errorf("read source control credentials: %w", err)
	}

	opened, err := r.open(token)
	if err != nil {
		return repository.SCMCredentials{}, err
	}

	signing, err := r.open(secret)
	if err != nil {
		return repository.SCMCredentials{}, err
	}

	return repository.SCMCredentials{Token: opened, WebhookSecret: signing}, nil
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
    reconcile_after = NULL,
    updated_at = $5
WHERE id = $1`

func (r *connectionRepository) ReplaceToken(
	ctx context.Context,
	connectionID uuid.UUID,
	token, hint, login string,
	at time.Time,
) error {
	sealed, err := r.seal(token)
	if err != nil {
		return err
	}

	result, err := r.db.Querier(ctx).
		ExecContext(ctx, replaceTokenQuery, connectionID, sealed, hint, login, at)
	if err != nil {
		return fmt.Errorf("replace source control token: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const updateSettingsQuery = `
UPDATE workspace_scm_connections
SET team_id = $2, mirror_label = $3, updated_at = now()
WHERE id = $1
RETURNING` + connectionColumns

func (r *connectionRepository) UpdateSettings(
	ctx context.Context,
	connectionID, teamID uuid.UUID,
	mirrorLabel string,
) (entity.SCMConnection, error) {
	connection, err := scanConnection(r.db.Querier(ctx).QueryRowContext(
		ctx,
		updateSettingsQuery,
		connectionID,
		teamOrNil(teamID),
		mirrorLabel,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMConnection{}, entity.ErrSCMConnectionNotFound
	}

	if err != nil {
		return entity.SCMConnection{}, fmt.Errorf("update source control connection: %w", err)
	}

	return connection, nil
}

const recordHookQuery = `
UPDATE workspace_scm_connections
SET external_hook_id = $2, updated_at = now()
WHERE id = $1`

func (r *connectionRepository) RecordHook(
	ctx context.Context,
	connectionID uuid.UUID,
	externalHookID string,
) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, recordHookQuery, connectionID, externalHookID)
	if err != nil {
		return fmt.Errorf("record source control hook: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const markVerifiedQuery = `
UPDATE workspace_scm_connections
SET external_repository_id = $2,
    identity_login = $3,
    status = 'connected',
    broken_reason = '',
    broken_detail = '',
    broken_at = NULL,
    verified_at = $4,
    updated_at = $4
WHERE id = $1`

func (r *connectionRepository) MarkVerified(
	ctx context.Context,
	connectionID uuid.UUID,
	repo entity.SCMRepository,
	login string,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).
		ExecContext(ctx, markVerifiedQuery, connectionID, repo.ExternalID, login, at)
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
    broken_at = coalesce(broken_at, $4),
    updated_at = $4
WHERE id = $1`

// MarkBroken keeps the first broken_at rather than restamping it, so the once-per-break
// notice stays once per break however many calls fail behind it.
func (r *connectionRepository) MarkBroken(
	ctx context.Context,
	connectionID uuid.UUID,
	reason entity.SCMBrokenReason,
	detail string,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).
		ExecContext(ctx, markBrokenQuery, connectionID, reason, detail, at)
	if err != nil {
		return fmt.Errorf("mark source control connection broken: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const parkQuery = `
UPDATE workspace_scm_connections
SET reconcile_after = $2, updated_at = now()
WHERE id = $1`

func (r *connectionRepository) Park(
	ctx context.Context,
	connectionID uuid.UUID,
	until time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, parkQuery, connectionID, until)
	if err != nil {
		return fmt.Errorf("park source control connection: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const recordReconciledQuery = `
UPDATE workspace_scm_connections
SET reconcile_cursor = $2, reconciled_at = $3, reconcile_after = NULL, updated_at = $3
WHERE id = $1`

func (r *connectionRepository) RecordReconciled(
	ctx context.Context,
	connectionID uuid.UUID,
	cursor string,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).
		ExecContext(ctx, recordReconciledQuery, connectionID, cursor, at)
	if err != nil {
		return fmt.Errorf("record source control reconcile: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const recordSeenQuery = `
UPDATE workspace_scm_connections
SET last_seen_at = $2
WHERE id = $1`

func (r *connectionRepository) RecordSeen(
	ctx context.Context,
	connectionID uuid.UUID,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, recordSeenQuery, connectionID, at); err != nil {
		return fmt.Errorf("record source control delivery seen: %w", err)
	}

	return nil
}

// claimDueQuery takes the connections that have gone longest without a reconcile, skipping
// any a parallel worker already holds and any parked by a rate limit. A broken connection is
// still claimed: the sweep is what notices a replaced token and repairs it, so excluding one
// would leave it broken until somebody happened to press a button.
const claimDueQuery = `
UPDATE workspace_scm_connections
SET reconciled_at = $1
WHERE id IN (
    SELECT id
    FROM workspace_scm_connections
    WHERE (reconcile_after IS NULL OR reconcile_after <= $1)
      AND (reconciled_at IS NULL OR reconciled_at < $2)
    ORDER BY reconciled_at NULLS FIRST, id
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
RETURNING` + connectionColumns

func (r *connectionRepository) ClaimDue(
	ctx context.Context,
	at time.Time,
	limit int,
) ([]entity.SCMConnection, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, claimDueQuery, at, at, limit)
	if err != nil {
		return nil, fmt.Errorf("claim source control connections: %w", err)
	}

	defer func() { _ = rows.Close() }()

	connections := make([]entity.SCMConnection, 0, limit)

	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			return nil, fmt.Errorf("scan claimed source control connection: %w", err)
		}

		connections = append(connections, connection)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read claimed source control connections: %w", err)
	}

	return connections, nil
}

const deleteConnectionQuery = `DELETE FROM workspace_scm_connections WHERE id = $1`

func (r *connectionRepository) Delete(ctx context.Context, connectionID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteConnectionQuery, connectionID)
	if err != nil {
		return fmt.Errorf("delete source control connection: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

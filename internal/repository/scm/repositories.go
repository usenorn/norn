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

type repositoryRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func NewSCMRepository(db *postgres.Client, sealer *crypter.Crypter) repository.SCMRepository {
	return &repositoryRepository{db: db, crypter: sealer}
}

const repositoryColumns = `
    id, connection_id, workspace_id, provider, full_name, external_id, default_branch, url,
    coalesce(octet_length(webhook_secret_sealed), 0) > 0, external_hook_id, mirror_label, sync_direction,
    webhooks_disabled,
    extract(epoch FROM poll_interval), reconcile_cursor, reconciled_at, reconcile_after,
    last_seen_at, backfilled_at, created_at, updated_at`

func scanRepository(row interface{ Scan(...any) error }) (entity.SCMRepository, error) {
	var (
		stored  entity.SCMRepository
		seconds float64
	)

	err := row.Scan(
		&stored.ID,
		&stored.ConnectionID,
		&stored.WorkspaceID,
		&stored.Provider,
		&stored.FullName,
		&stored.ExternalID,
		&stored.DefaultBranch,
		&stored.URL,
		&stored.WebhookSecretSet,
		&stored.ExternalHookID,
		&stored.MirrorLabel,
		&stored.SyncDirection,
		&stored.WebhooksDisabled,
		&seconds,
		&stored.ReconcileCursor,
		&stored.ReconciledAt,
		&stored.ReconcileAfter,
		&stored.LastSeenAt,
		&stored.BackfilledAt,
		&stored.CreatedAt,
		&stored.UpdatedAt,
	)
	if err != nil {
		return entity.SCMRepository{}, err
	}

	stored.PollInterval = time.Duration(seconds * float64(time.Second))

	return stored, nil
}

const insertRepositoryQuery = `
INSERT INTO workspace_scm_repositories (
    id, connection_id, workspace_id, provider, full_name, external_id, default_branch, url,
    webhook_secret_sealed, mirror_label, sync_direction, poll_interval
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, make_interval(secs => $12))
RETURNING` + repositoryColumns

func (r *repositoryRepository) Create(
	ctx context.Context,
	input repository.SCMRepositoryInput,
) (entity.SCMRepository, error) {
	stored := input.Repository

	if stored.ID == uuid.Nil {
		stored.ID = uuid.New()
	}

	if stored.PollInterval <= 0 {
		stored.PollInterval = entity.SCMDefaultPollInterval
	}

	var (
		secret []byte
		err    error
	)

	if input.WebhookSecret != "" {
		if secret, err = seal(r.crypter, input.WebhookSecret); err != nil {
			return entity.SCMRepository{}, err
		}
	}

	created, err := scanRepository(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertRepositoryQuery,
		stored.ID,
		stored.ConnectionID,
		stored.WorkspaceID,
		stored.Provider,
		stored.FullName,
		stored.ExternalID,
		stored.DefaultBranch,
		stored.URL,
		secret,
		stored.MirrorLabel,
		stored.Direction(),
		stored.PollInterval.Seconds(),
	))
	if err != nil {
		if violates(err, repositoryNameUniqueIndex) {
			return entity.SCMRepository{}, entity.ErrSCMRepositoryExists
		}

		return entity.SCMRepository{}, fmt.Errorf("connect repository: %w", err)
	}

	return created, nil
}

const getRepositoryQuery = `
SELECT` + repositoryColumns + `
FROM workspace_scm_repositories
WHERE workspace_id = $1 AND id = $2`

func (r *repositoryRepository) GetByID(
	ctx context.Context,
	workspaceID, repositoryID uuid.UUID,
) (entity.SCMRepository, error) {
	stored, err := scanRepository(
		r.db.Querier(ctx).QueryRowContext(ctx, getRepositoryQuery, workspaceID, repositoryID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMRepository{}, entity.ErrSCMRepositoryNotFound
	}

	if err != nil {
		return entity.SCMRepository{}, fmt.Errorf("read connected repository: %w", err)
	}

	return stored, nil
}

const getRepositoryByFullNameQuery = `
SELECT` + repositoryColumns + `
FROM workspace_scm_repositories
WHERE connection_id = $1 AND lower(full_name) = lower($2)`

func (r *repositoryRepository) GetByFullName(
	ctx context.Context,
	connectionID uuid.UUID,
	fullName string,
) (entity.SCMRepository, error) {
	stored, err := scanRepository(r.db.Querier(ctx).QueryRowContext(
		ctx, getRepositoryByFullNameQuery, connectionID, fullName,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMRepository{}, entity.ErrSCMRepositoryNotFound
	}

	if err != nil {
		return entity.SCMRepository{}, fmt.Errorf("read source control repository: %w", err)
	}

	return stored, nil
}

const getRepositoryForDeliveryQuery = `
SELECT` + repositoryColumns + `
FROM workspace_scm_repositories
WHERE id = $1`

func (r *repositoryRepository) GetForDelivery(
	ctx context.Context,
	repositoryID uuid.UUID,
) (entity.SCMRepository, error) {
	stored, err := scanRepository(
		r.db.Querier(ctx).QueryRowContext(ctx, getRepositoryForDeliveryQuery, repositoryID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMRepository{}, entity.ErrSCMRepositoryNotFound
	}

	if err != nil {
		return entity.SCMRepository{}, fmt.Errorf("read connected repository: %w", err)
	}

	return stored, nil
}

const listRepositoriesByConnectionQuery = `
SELECT` + repositoryColumns + `
FROM workspace_scm_repositories
WHERE connection_id = $1
ORDER BY full_name`

func (r *repositoryRepository) ListByConnection(
	ctx context.Context,
	connectionID uuid.UUID,
) ([]entity.SCMRepository, error) {
	return r.list(ctx, listRepositoriesByConnectionQuery, connectionID)
}

const listRepositoriesByWorkspaceQuery = `
SELECT` + repositoryColumns + `
FROM workspace_scm_repositories
WHERE workspace_id = $1
ORDER BY full_name`

func (r *repositoryRepository) ListByWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.SCMRepository, error) {
	return r.list(ctx, listRepositoriesByWorkspaceQuery, workspaceID)
}

func (r *repositoryRepository) list(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.SCMRepository, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list connected repositories: %w", err)
	}

	defer func() { _ = rows.Close() }()

	stored := make([]entity.SCMRepository, 0)

	for rows.Next() {
		one, err := scanRepository(rows)
		if err != nil {
			return nil, fmt.Errorf("read connected repository: %w", err)
		}

		stored = append(stored, one)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list connected repositories: %w", err)
	}

	return stored, nil
}

const webhookSecretQuery = `
SELECT webhook_secret_sealed
FROM workspace_scm_repositories
WHERE id = $1`

func (r *repositoryRepository) WebhookSecret(
	ctx context.Context,
	repositoryID uuid.UUID,
) (string, error) {
	var sealed []byte

	err := r.db.Querier(ctx).QueryRowContext(ctx, webhookSecretQuery, repositoryID).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return "", entity.ErrSCMRepositoryNotFound
	}

	if err != nil {
		return "", fmt.Errorf("read repository webhook secret: %w", err)
	}

	return open(r.crypter, sealed)
}

const updateRepositorySettingsQuery = `
UPDATE workspace_scm_repositories
SET mirror_label = $2,
    sync_direction = $3,
    webhooks_disabled = $4,
    poll_interval = make_interval(secs => $5),
    updated_at = now()
WHERE id = $1
RETURNING` + repositoryColumns

func (r *repositoryRepository) UpdateSettings(
	ctx context.Context,
	repositoryID uuid.UUID,
	settings repository.SCMRepositorySettings,
) (entity.SCMRepository, error) {
	interval := settings.PollInterval
	if interval <= 0 {
		interval = entity.SCMDefaultPollInterval
	}

	direction := settings.SyncDirection
	if !direction.Valid() || direction == "" {
		direction = entity.MirrorBoth
	}

	stored, err := scanRepository(r.db.Querier(ctx).QueryRowContext(
		ctx,
		updateRepositorySettingsQuery,
		repositoryID,
		settings.MirrorLabel,
		direction,
		settings.WebhooksDisabled,
		interval.Seconds(),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMRepository{}, entity.ErrSCMRepositoryNotFound
	}

	if err != nil {
		return entity.SCMRepository{}, fmt.Errorf("update connected repository: %w", err)
	}

	return stored, nil
}

const recordHookQuery = `
UPDATE workspace_scm_repositories
SET external_hook_id = $2, updated_at = now()
WHERE id = $1`

func (r *repositoryRepository) RecordHook(
	ctx context.Context,
	repositoryID uuid.UUID,
	externalHookID string,
) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, recordHookQuery, repositoryID, externalHookID)
	if err != nil {
		return fmt.Errorf("record repository webhook: %w", err)
	}

	return expectOne(result, entity.ErrSCMRepositoryNotFound)
}

const recordSeenQuery = `
UPDATE workspace_scm_repositories
SET last_seen_at = $2, updated_at = now()
WHERE id = $1`

func (r *repositoryRepository) RecordSeen(
	ctx context.Context,
	repositoryID uuid.UUID,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, recordSeenQuery, repositoryID, at)
	if err != nil {
		return fmt.Errorf("record repository delivery: %w", err)
	}

	return expectOne(result, entity.ErrSCMRepositoryNotFound)
}

const recordReconciledQuery = `
UPDATE workspace_scm_repositories
SET reconcile_cursor = $2, reconciled_at = $3, reconcile_after = NULL, updated_at = now()
WHERE id = $1`

func (r *repositoryRepository) RecordReconciled(
	ctx context.Context,
	repositoryID uuid.UUID,
	cursor string,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, recordReconciledQuery, repositoryID, cursor, at,
	)
	if err != nil {
		return fmt.Errorf("record repository sweep: %w", err)
	}

	return expectOne(result, entity.ErrSCMRepositoryNotFound)
}

const parkQuery = `
UPDATE workspace_scm_repositories
SET reconcile_after = $2, updated_at = now()
WHERE id = $1`

func (r *repositoryRepository) Park(
	ctx context.Context,
	repositoryID uuid.UUID,
	until time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, parkQuery, repositoryID, until)
	if err != nil {
		return fmt.Errorf("park repository: %w", err)
	}

	return expectOne(result, entity.ErrSCMRepositoryNotFound)
}

const claimDueQuery = `
UPDATE workspace_scm_repositories AS r
SET reconciled_at = $1, updated_at = now()
FROM (
    SELECT d.id AS due_id
    FROM workspace_scm_repositories AS d
    JOIN workspace_scm_connections AS c ON c.id = d.connection_id
    WHERE c.status = 'connected'
      AND (d.reconcile_after IS NULL OR d.reconcile_after <= $1)
      AND (d.reconciled_at IS NULL OR d.reconciled_at + d.poll_interval <= $1)
    ORDER BY d.reconciled_at NULLS FIRST, d.id
    LIMIT $2
    FOR UPDATE OF d SKIP LOCKED
) AS due
WHERE r.id = due.due_id
RETURNING` + repositoryColumns

func (r *repositoryRepository) ClaimDue(
	ctx context.Context,
	at time.Time,
	limit int,
) ([]entity.SCMRepository, error) {
	return r.list(ctx, claimDueQuery, at, limit)
}

const deleteRepositoryQuery = `
DELETE FROM workspace_scm_repositories
WHERE workspace_id = $1 AND id = $2`

func (r *repositoryRepository) Delete(
	ctx context.Context,
	workspaceID, repositoryID uuid.UUID,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, deleteRepositoryQuery, workspaceID, repositoryID,
	)
	if err != nil {
		return fmt.Errorf("disconnect repository: %w", err)
	}

	return expectOne(result, entity.ErrSCMRepositoryNotFound)
}

const recordBackfilledQuery = `
UPDATE workspace_scm_repositories
SET backfilled_at = $2, updated_at = now()
WHERE id = $1`

func (r *repositoryRepository) RecordBackfilled(
	ctx context.Context,
	repositoryID uuid.UUID,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, recordBackfilledQuery, repositoryID, at)
	if err != nil {
		return fmt.Errorf("record a repository backfill: %w", err)
	}

	return expectOne(result, entity.ErrSCMRepositoryNotFound)
}

package imports

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const runColumns = `
SELECT id, workspace_id, source_kind, source_label,
       coalesce(requested_by_account_id::text, ''), requested_actor_kind, requested_auth_method,
       coalesce(reverted_by_account_id::text, ''), reverted_actor_kind, reverted_auth_method,
       status, phase_error, preview_digest, acknowledge_triage,
       source_settings, unknown_reference_policy, source_secret_sealed IS NOT NULL,
       coalesce(lease_token::text, ''), lease_expires_at, attempt, staged, processed,
       staged_at, mapped_at, started_at, finished_at, reverted_at, created_at, updated_at
FROM workspace_import_runs`

const runsByWorkspaceQuery = runColumns + `
WHERE workspace_id = $1
  AND ($2 = '' OR (created_at, id) < ($3::timestamptz, $2::uuid))
ORDER BY created_at DESC, id DESC
LIMIT $4`

const insertRunQuery = `
INSERT INTO workspace_import_runs (
    id, workspace_id, source_kind, source_label,
    requested_by_account_id, requested_actor_kind, requested_auth_method,
    status, preview_digest, acknowledge_triage,
    source_settings, unknown_reference_policy, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

const saveSourceConfigQuery = `
UPDATE workspace_import_runs
SET source_secret_sealed = coalesce($2, source_secret_sealed),
    source_settings = $3,
    unknown_reference_policy = $4,
    updated_at = now()
WHERE id = $1`

const sourceConfigQuery = `
SELECT source_secret_sealed, source_settings
FROM workspace_import_runs
WHERE id = $1`

const runByIDQuery = runColumns + `
WHERE id = $1 AND workspace_id = $2`

const claimRunQuery = `
UPDATE workspace_import_runs
SET status = $3,
    lease_token = $4,
    lease_expires_at = $5,
    attempt = attempt + 1,
    started_at = coalesce(started_at, $6),
    updated_at = $6
WHERE id = $1
  AND (status = $2
       OR (status = $3 AND (lease_expires_at IS NULL OR lease_expires_at < $6)))
RETURNING lease_token::text`

const heartbeatRunQuery = `
UPDATE workspace_import_runs
SET lease_expires_at = $3, updated_at = $3
WHERE id = $1 AND lease_token = $2`

const releaseRunQuery = `
UPDATE workspace_import_runs
SET lease_token = NULL, lease_expires_at = NULL, updated_at = $3
WHERE id = $1 AND lease_token = $2`

const advanceRunQuery = `
UPDATE workspace_import_runs
SET staged = staged + $2, processed = processed + $3, updated_at = now()
WHERE id = $1`

const settleRunQuery = `
UPDATE workspace_import_runs
SET status = $2,
    phase_error = $3,
    lease_token = NULL,
    lease_expires_at = NULL,
    staged_at = CASE WHEN $2 = 'staged' THEN $4 ELSE staged_at END,
    mapped_at = CASE WHEN $2 = 'mapped' THEN $4 ELSE mapped_at END,
    finished_at = CASE WHEN $2 IN ('imported', 'failed') THEN $4 ELSE finished_at END,
    reverted_at = CASE WHEN $2 = 'reverted' THEN $4 ELSE reverted_at END,
    updated_at = $4
WHERE id = $1`

const acceptPreviewQuery = `
UPDATE workspace_import_runs
SET preview_digest = $2, acknowledge_triage = $3, updated_at = $4
WHERE id = $1`

const recordReverterQuery = `
UPDATE workspace_import_runs
SET reverted_by_account_id = $2,
    reverted_actor_kind = $3,
    reverted_auth_method = $4,
    updated_at = $5
WHERE id = $1`

// A released run holds no lease, so its expiry is NULL and an expiry test alone cannot
// see it. That is the state a slice leaves behind between finishing and its own
// re-enqueue, and if the enqueue is lost the run would otherwise sit in executing
// forever with nothing watching it. The second disjunct is what closes that.
const leaseExpiredQuery = runColumns + `
WHERE status IN ('staging', 'executing', 'reverting')
  AND ((lease_expires_at IS NOT NULL AND lease_expires_at < $1)
       OR (lease_expires_at IS NULL AND updated_at < $2))
ORDER BY coalesce(lease_expires_at, updated_at), id
LIMIT $3
FOR UPDATE SKIP LOCKED`

type runRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func NewImportRun(db *postgres.Client, sealer *crypter.Crypter) repository.ImportRun {
	return &runRepository{db: db, crypter: sealer}
}

func (r *runRepository) seal(secret string) ([]byte, error) {
	sealed, err := r.crypter.Seal([]byte(secret))
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return nil, entity.ErrImportEncryptionKeyMissing
		}

		return nil, fmt.Errorf("seal import source key: %w", err)
	}

	return sealed, nil
}

func (r *runRepository) open(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}

	secret, err := r.crypter.Open(sealed)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return "", entity.ErrImportEncryptionKeyMissing
		}

		return "", fmt.Errorf("open stored import source key: %w", err)
	}

	return string(secret), nil
}

func (r *runRepository) Create(
	ctx context.Context,
	run entity.ImportRun,
) (entity.ImportRun, error) {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}

	if run.Status == "" {
		run.Status = entity.ImportDraft
	}

	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}

	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		insertRunQuery,
		run.ID.String(),
		run.WorkspaceID.String(),
		run.SourceKind,
		run.SourceLabel,
		optionalID(run.RequestedByAccount),
		string(run.RequestedActorKind),
		string(run.RequestedAuthMethod),
		string(run.Status),
		run.PreviewDigest,
		run.AcknowledgeTriage,
		settingsOf(run.Settings),
		string(run.UnknownReferences.Or(entity.ImportUnknownSkip)),
		run.CreatedAt,
		run.UpdatedAt,
	); err != nil {
		return entity.ImportRun{}, fmt.Errorf("insert import run: %w", err)
	}

	return r.GetByID(ctx, run.WorkspaceID, run.ID)
}

func (r *runRepository) GetByID(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
) (entity.ImportRun, error) {
	run, err := scanRun(r.db.Querier(ctx).QueryRowContext(
		ctx, runByIDQuery, runID.String(), workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ImportRun{}, entity.ErrImportRunNotFound
		}

		return entity.ImportRun{}, fmt.Errorf("find import run: %w", err)
	}

	return run, nil
}

func (r *runRepository) ListByWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
	page entity.ImportRunPage,
) ([]entity.ImportRun, error) {
	page = page.Normalized()

	cursor, at := "", time.Time{}

	if page.Cursor != nil {
		cursor, at = page.Cursor.RunID.String(), page.Cursor.CreatedAt
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, runsByWorkspaceQuery, workspaceID.String(), cursor, at, page.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list import runs: %w", err)
	}

	defer func() { _ = rows.Close() }()

	runs := make([]entity.ImportRun, 0, page.Limit)

	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan import run: %w", err)
		}

		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read import runs: %w", err)
	}

	return runs, nil
}

func (r *runRepository) SaveSourceConfig(
	ctx context.Context,
	runID uuid.UUID,
	secret string,
	settings json.RawMessage,
	policy entity.ImportUnknownPolicy,
) error {
	var sealed any

	if secret != "" {
		ciphertext, err := r.seal(secret)
		if err != nil {
			return err
		}

		sealed = ciphertext
	}

	saved, err := r.affected(
		ctx,
		saveSourceConfigQuery,
		runID.String(),
		sealed,
		settingsOf(settings),
		string(policy.Or(entity.ImportUnknownSkip)),
	)
	if err != nil {
		return fmt.Errorf("save import source config: %w", err)
	}

	if saved == 0 {
		return entity.ErrImportRunNotFound
	}

	return nil
}

func (r *runRepository) SourceConfigForStaging(
	ctx context.Context,
	runID uuid.UUID,
) (repository.ImportSourceSecret, error) {
	var (
		sealed   []byte
		settings string
	)

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, sourceConfigQuery, runID.String(),
	).Scan(&sealed, &settings); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.ImportSourceSecret{}, entity.ErrImportRunNotFound
		}

		return repository.ImportSourceSecret{}, fmt.Errorf("read import source config: %w", err)
	}

	secret, err := r.open(sealed)
	if err != nil {
		return repository.ImportSourceSecret{}, err
	}

	return repository.ImportSourceSecret{
		Secret:   secret,
		Settings: json.RawMessage(settings),
	}, nil
}

func (r *runRepository) Claim(
	ctx context.Context,
	runID uuid.UUID,
	from, to entity.ImportStatus,
	lease repository.ImportLease,
	at time.Time,
) (uuid.UUID, error) {
	token := lease.Token
	if token == uuid.Nil {
		token = uuid.New()
	}

	var claimed string

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		claimRunQuery,
		runID.String(),
		string(from),
		string(to),
		token.String(),
		lease.Expires,
		at,
	).Scan(&claimed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, entity.ErrImportRunLeased
		}

		return uuid.Nil, fmt.Errorf("claim import run: %w", err)
	}

	return parseID(claimed), nil
}

func (r *runRepository) Heartbeat(
	ctx context.Context,
	runID, token uuid.UUID,
	expires time.Time,
) error {
	held, err := r.affected(ctx, heartbeatRunQuery, runID.String(), token.String(), expires)
	if err != nil {
		return fmt.Errorf("heartbeat import run: %w", err)
	}

	if held == 0 {
		return entity.ErrImportRunLeased
	}

	return nil
}

func (r *runRepository) Release(ctx context.Context, runID, token uuid.UUID, at time.Time) error {
	if _, err := r.affected(ctx, releaseRunQuery, runID.String(), token.String(), at); err != nil {
		return fmt.Errorf("release import run: %w", err)
	}

	return nil
}

func (r *runRepository) Advance(ctx context.Context, runID uuid.UUID, staged, processed int) error {
	if _, err := r.affected(ctx, advanceRunQuery, runID.String(), staged, processed); err != nil {
		return fmt.Errorf("advance import run: %w", err)
	}

	return nil
}

func (r *runRepository) Settle(
	ctx context.Context,
	runID uuid.UUID,
	status entity.ImportStatus,
	detail string,
	at time.Time,
) error {
	settled, err := r.affected(ctx, settleRunQuery, runID.String(), string(status), detail, at)
	if err != nil {
		return fmt.Errorf("settle import run: %w", err)
	}

	if settled == 0 {
		return entity.ErrImportRunNotFound
	}

	return nil
}

func (r *runRepository) AcceptPreview(
	ctx context.Context,
	runID uuid.UUID,
	digest string,
	acknowledgeTriage bool,
	at time.Time,
) error {
	accepted, err := r.affected(
		ctx, acceptPreviewQuery, runID.String(), digest, acknowledgeTriage, at,
	)
	if err != nil {
		return fmt.Errorf("accept import preview: %w", err)
	}

	if accepted == 0 {
		return entity.ErrImportRunNotFound
	}

	return nil
}

func (r *runRepository) RecordReverter(
	ctx context.Context,
	runID uuid.UUID,
	actor entity.Actor,
	at time.Time,
) error {
	recorded, err := r.affected(
		ctx,
		recordReverterQuery,
		runID.String(),
		optionalID(actor.AccountID),
		string(actor.Kind),
		string(actor.AuthMethod),
		at,
	)
	if err != nil {
		return fmt.Errorf("record import reverter: %w", err)
	}

	if recorded == 0 {
		return entity.ErrImportRunNotFound
	}

	return nil
}

func (r *runRepository) ListLeaseExpired(
	ctx context.Context,
	at, idleSince time.Time,
	limit int,
) ([]entity.ImportRun, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, leaseExpiredQuery, at, idleSince, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired import leases: %w", err)
	}

	defer func() { _ = rows.Close() }()

	runs := make([]entity.ImportRun, 0, limit)

	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan import run: %w", err)
		}

		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read expired import leases: %w", err)
	}

	return runs, nil
}

func (r *runRepository) affected(ctx context.Context, query string, args ...any) (int64, error) {
	result, err := r.db.Querier(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRun(row scanner) (entity.ImportRun, error) {
	var (
		run                             entity.ImportRun
		id, workspaceID                 string
		requestedBy, requestedKind      string
		requestedMethod, revertedBy     string
		revertedKind, revertedMethod    string
		status, leaseToken              string
		settings, unknownReferences     string
		leaseExpires, stagedAt          sql.NullTime
		mappedAt, startedAt, finishedAt sql.NullTime
		revertedAt                      sql.NullTime
	)

	if err := row.Scan(
		&id, &workspaceID, &run.SourceKind, &run.SourceLabel,
		&requestedBy, &requestedKind, &requestedMethod,
		&revertedBy, &revertedKind, &revertedMethod,
		&status, &run.PhaseError, &run.PreviewDigest, &run.AcknowledgeTriage,
		&settings, &unknownReferences, &run.SourceSecretSet,
		&leaseToken, &leaseExpires, &run.Attempt, &run.Staged, &run.Processed,
		&stagedAt, &mappedAt, &startedAt, &finishedAt, &revertedAt,
		&run.CreatedAt, &run.UpdatedAt,
	); err != nil {
		return entity.ImportRun{}, err
	}

	run.ID = parseID(id)
	run.WorkspaceID = parseID(workspaceID)
	run.RequestedByAccount = parseID(requestedBy)
	run.RequestedActorKind = entity.ActorKind(requestedKind)
	run.RequestedAuthMethod = entity.SessionAuthMethod(requestedMethod)
	run.RevertedByAccount = parseID(revertedBy)
	run.RevertedActorKind = entity.ActorKind(revertedKind)
	run.RevertedAuthMethod = entity.SessionAuthMethod(revertedMethod)
	run.Status = entity.ImportStatus(status)
	run.Settings = json.RawMessage(settings)
	run.UnknownReferences = entity.ImportUnknownPolicy(unknownReferences)
	run.LeaseToken = parseID(leaseToken)
	run.LeaseExpiresAt = optionalTime(leaseExpires)
	run.StagedAt = optionalTime(stagedAt)
	run.MappedAt = optionalTime(mappedAt)
	run.StartedAt = optionalTime(startedAt)
	run.FinishedAt = optionalTime(finishedAt)
	run.RevertedAt = optionalTime(revertedAt)

	return run, nil
}

func parseID(raw string) uuid.UUID {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}

	return parsed
}

func settingsOf(settings json.RawMessage) string {
	if len(settings) == 0 {
		return "{}"
	}

	return string(settings)
}

func optionalID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}

	return id.String()
}

func optionalTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}

	at := value.Time

	return &at
}

func optionalIdentifier(id uuid.UUID) *string {
	if id == uuid.Nil {
		return nil
	}

	value := id.String()

	return &value
}

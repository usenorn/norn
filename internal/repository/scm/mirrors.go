package scm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type mirrorRepository struct {
	db *postgres.Client
}

func NewIssueMirror(db *postgres.Client) repository.IssueMirror {
	return &mirrorRepository{db: db}
}

const mirrorColumns = `
    id, workspace_id, issue_id,
    coalesce(connection_id, '00000000-0000-0000-0000-000000000000'::uuid),
    provider, repository, external_id, external_number, url, origin,
    title_hash, body_hash, state_hash, synced_version, source_updated_at,
    pulled_at, pushed_at, created_at, updated_at`

func scanMirror(row interface{ Scan(...any) error }) (entity.IssueMirror, error) {
	var mirror entity.IssueMirror

	err := row.Scan(
		&mirror.ID,
		&mirror.WorkspaceID,
		&mirror.IssueID,
		&mirror.ConnectionID,
		&mirror.Provider,
		&mirror.Repository,
		&mirror.ExternalID,
		&mirror.ExternalNumber,
		&mirror.URL,
		&mirror.Origin,
		&mirror.TitleHash,
		&mirror.BodyHash,
		&mirror.StateHash,
		&mirror.SyncedVersion,
		&mirror.SourceUpdatedAt,
		&mirror.PulledAt,
		&mirror.PushedAt,
		&mirror.CreatedAt,
		&mirror.UpdatedAt,
	)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	return mirror, nil
}

const insertMirrorQuery = `
INSERT INTO workspace_issue_mirrors (
    id, workspace_id, issue_id, connection_id, provider, repository,
    external_id, external_number, url, origin
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING` + mirrorColumns

func (r *mirrorRepository) Create(
	ctx context.Context,
	mirror entity.IssueMirror,
) (entity.IssueMirror, error) {
	if mirror.ID == uuid.Nil {
		mirror.ID = uuid.New()
	}

	created, err := scanMirror(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertMirrorQuery,
		mirror.ID,
		mirror.WorkspaceID,
		mirror.IssueID,
		connectionOrNil(mirror.ConnectionID),
		mirror.Provider,
		mirror.Repository,
		mirror.ExternalID,
		mirror.ExternalNumber,
		mirror.URL,
		mirror.Origin,
	))
	if err != nil {
		if violates(err, mirrorIssueUniqueIndex) || violates(err, mirrorExternalUniqueIndex) {
			return entity.IssueMirror{}, entity.ErrIssueMirrorExists
		}

		return entity.IssueMirror{}, fmt.Errorf("mirror an issue: %w", err)
	}

	return created, nil
}

const getMirrorByIssueQuery = `
SELECT` + mirrorColumns + `
FROM workspace_issue_mirrors
WHERE workspace_id = $1 AND issue_id = $2`

func (r *mirrorRepository) GetByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.IssueMirror, error) {
	return mirrorOrFail(
		r.db.Querier(ctx).QueryRowContext(ctx, getMirrorByIssueQuery, workspaceID, issueID),
	)
}

const getMirrorByExternalQuery = `
SELECT` + mirrorColumns + `
FROM workspace_issue_mirrors
WHERE workspace_id = $1 AND provider = $2 AND repository = $3 AND external_id = $4`

func (r *mirrorRepository) GetByExternalID(
	ctx context.Context,
	workspaceID uuid.UUID,
	provider entity.SCMProvider,
	repo, externalID string,
) (entity.IssueMirror, error) {
	return mirrorOrFail(r.db.Querier(ctx).QueryRowContext(
		ctx,
		getMirrorByExternalQuery,
		workspaceID,
		provider,
		repo,
		externalID,
	))
}

func mirrorOrFail(row interface{ Scan(...any) error }) (entity.IssueMirror, error) {
	mirror, err := scanMirror(row)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.IssueMirror{}, entity.ErrIssueMirrorNotFound
	}

	if err != nil {
		return entity.IssueMirror{}, fmt.Errorf("read issue mirror: %w", err)
	}

	return mirror, nil
}

const listMirrorsByConnectionQuery = `
SELECT` + mirrorColumns + `
FROM workspace_issue_mirrors
WHERE connection_id = $1
ORDER BY coalesce(pulled_at, created_at), id
LIMIT $2`

func (r *mirrorRepository) ListByConnection(
	ctx context.Context,
	connectionID uuid.UUID,
	limit int,
) ([]entity.IssueMirror, error) {
	rows, err := r.db.Querier(ctx).
		QueryContext(ctx, listMirrorsByConnectionQuery, connectionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list issue mirrors: %w", err)
	}

	defer func() { _ = rows.Close() }()

	mirrors := make([]entity.IssueMirror, 0, limit)

	for rows.Next() {
		mirror, err := scanMirror(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issue mirror: %w", err)
		}

		mirrors = append(mirrors, mirror)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read issue mirrors: %w", err)
	}

	return mirrors, nil
}

const recordPullQuery = `
UPDATE workspace_issue_mirrors
SET title_hash = $2, body_hash = $3, state_hash = $4, synced_version = $5,
    source_updated_at = $6, pulled_at = $7, updated_at = $7
WHERE id = $1`

func (r *mirrorRepository) RecordPull(
	ctx context.Context,
	mirrorID uuid.UUID,
	hashes entity.MirrorHashes,
	sourceUpdatedAt time.Time,
	version int,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordPullQuery,
		mirrorID,
		hashes.Title,
		hashes.Body,
		hashes.State,
		version,
		sourceUpdatedAt,
		at,
	)
	if err != nil {
		return fmt.Errorf("record a mirror pull: %w", err)
	}

	return expectOne(result, entity.ErrIssueMirrorNotFound)
}

const recordPushQuery = `
UPDATE workspace_issue_mirrors
SET title_hash = $2, body_hash = $3, state_hash = $4, synced_version = $5,
    pushed_at = $6, updated_at = $6
WHERE id = $1`

func (r *mirrorRepository) RecordPush(
	ctx context.Context,
	mirrorID uuid.UUID,
	hashes entity.MirrorHashes,
	version int,
	at time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordPushQuery,
		mirrorID,
		hashes.Title,
		hashes.Body,
		hashes.State,
		version,
		at,
	)
	if err != nil {
		return fmt.Errorf("record a mirror push: %w", err)
	}

	return expectOne(result, entity.ErrIssueMirrorNotFound)
}

const deleteMirrorQuery = `
DELETE FROM workspace_issue_mirrors WHERE workspace_id = $1 AND issue_id = $2`

func (r *mirrorRepository) Delete(ctx context.Context, workspaceID, issueID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteMirrorQuery, workspaceID, issueID)
	if err != nil {
		return fmt.Errorf("stop mirroring an issue: %w", err)
	}

	return expectOne(result, entity.ErrIssueMirrorNotFound)
}

const detachMirrorsQuery = `
UPDATE workspace_issue_mirrors
SET connection_id = NULL, updated_at = now()
WHERE connection_id = $1`

func (r *mirrorRepository) DetachConnection(ctx context.Context, connectionID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, detachMirrorsQuery, connectionID); err != nil {
		return fmt.Errorf("detach issue mirrors from a connection: %w", err)
	}

	return nil
}

const commentMirrorColumns = `
    id, workspace_id, issue_id, comment_id,
    coalesce(connection_id, '00000000-0000-0000-0000-000000000000'::uuid),
    provider, repository, external_id, external_author, origin, body_hash,
    source_updated_at, created_at, updated_at`

func scanCommentMirror(row interface{ Scan(...any) error }) (entity.CommentMirror, error) {
	var mirror entity.CommentMirror

	err := row.Scan(
		&mirror.ID,
		&mirror.WorkspaceID,
		&mirror.IssueID,
		&mirror.CommentID,
		&mirror.ConnectionID,
		&mirror.Provider,
		&mirror.Repository,
		&mirror.ExternalID,
		&mirror.ExternalAuthor,
		&mirror.Origin,
		&mirror.BodyHash,
		&mirror.SourceUpdatedAt,
		&mirror.CreatedAt,
		&mirror.UpdatedAt,
	)
	if err != nil {
		return entity.CommentMirror{}, err
	}

	return mirror, nil
}

const insertCommentMirrorQuery = `
INSERT INTO workspace_comment_mirrors (
    id, workspace_id, issue_id, comment_id, connection_id, provider, repository,
    external_id, external_author, origin, body_hash, source_updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING` + commentMirrorColumns

func (r *mirrorRepository) CreateComment(
	ctx context.Context,
	mirror entity.CommentMirror,
) (entity.CommentMirror, error) {
	if mirror.ID == uuid.Nil {
		mirror.ID = uuid.New()
	}

	created, err := scanCommentMirror(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertCommentMirrorQuery,
		mirror.ID,
		mirror.WorkspaceID,
		mirror.IssueID,
		mirror.CommentID,
		connectionOrNil(mirror.ConnectionID),
		mirror.Provider,
		mirror.Repository,
		mirror.ExternalID,
		mirror.ExternalAuthor,
		mirror.Origin,
		mirror.BodyHash,
		mirror.SourceUpdatedAt,
	))
	if err != nil {
		return entity.CommentMirror{}, fmt.Errorf("mirror a comment: %w", err)
	}

	return created, nil
}

const getCommentMirrorQuery = `
SELECT` + commentMirrorColumns + `
FROM workspace_comment_mirrors
WHERE workspace_id = $1 AND provider = $2 AND repository = $3 AND external_id = $4`

func (r *mirrorRepository) GetCommentByExternalID(
	ctx context.Context,
	workspaceID uuid.UUID,
	provider entity.SCMProvider,
	repo, externalID string,
) (entity.CommentMirror, error) {
	mirror, err := scanCommentMirror(r.db.Querier(ctx).QueryRowContext(
		ctx,
		getCommentMirrorQuery,
		workspaceID,
		provider,
		repo,
		externalID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.CommentMirror{}, entity.ErrIssueMirrorNotFound
	}

	if err != nil {
		return entity.CommentMirror{}, fmt.Errorf("read comment mirror: %w", err)
	}

	return mirror, nil
}

const listCommentMirrorsQuery = `
SELECT` + commentMirrorColumns + `
FROM workspace_comment_mirrors
WHERE workspace_id = $1 AND issue_id = $2
ORDER BY created_at, id`

func (r *mirrorRepository) ListCommentsByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.CommentMirror, error) {
	rows, err := r.db.Querier(ctx).
		QueryContext(ctx, listCommentMirrorsQuery, workspaceID, issueID)
	if err != nil {
		return nil, fmt.Errorf("list comment mirrors: %w", err)
	}

	defer func() { _ = rows.Close() }()

	mirrors := make([]entity.CommentMirror, 0)

	for rows.Next() {
		mirror, err := scanCommentMirror(rows)
		if err != nil {
			return nil, fmt.Errorf("scan comment mirror: %w", err)
		}

		mirrors = append(mirrors, mirror)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read comment mirrors: %w", err)
	}

	return mirrors, nil
}

const detachCommentMirrorsQuery = `
UPDATE workspace_comment_mirrors
SET connection_id = NULL, updated_at = now()
WHERE connection_id = $1`

func (r *mirrorRepository) DetachConnectionComments(
	ctx context.Context,
	connectionID uuid.UUID,
) error {
	_, err := r.db.Querier(ctx).ExecContext(ctx, detachCommentMirrorsQuery, connectionID)
	if err != nil {
		return fmt.Errorf("detach comment mirrors from a connection: %w", err)
	}

	return nil
}

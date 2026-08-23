package previewshare

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

const linkColumns = `
       id,
       preview_id,
       execution_id,
       workspace_id,
       token_hash,
       passcode_hash,
       coalesce(created_by::text, ''),
       expires_at,
       revoked_at,
       last_used_at,
       uses,
       created_at,
       updated_at`

const createLinkQuery = `
WITH created AS (
    INSERT INTO workspace_execution_preview_links
        (preview_id, execution_id, workspace_id, token_hash, passcode_hash, created_by, expires_at)
    VALUES ($1, $2, $3, $4, $5, nullif($6, '')::uuid, $7)
    RETURNING *
)
SELECT` + linkColumns + `
FROM created`

const linkByTokenQuery = `
SELECT` + linkColumns + `
FROM workspace_execution_preview_links
WHERE token_hash = $1`

const linksByPreviewQuery = `
SELECT` + linkColumns + `
FROM workspace_execution_preview_links
WHERE preview_id = $1
ORDER BY created_at, id`

const linksByExecutionQuery = `
SELECT` + linkColumns + `
FROM workspace_execution_preview_links
WHERE execution_id = $1
ORDER BY created_at, id`

const revokeLinkQuery = `
WITH revoked AS (
    UPDATE workspace_execution_preview_links
    SET revoked_at = coalesce(revoked_at, $3), updated_at = now()
    WHERE id = $2 AND preview_id = $1
    RETURNING *
)
SELECT` + linkColumns + `
FROM revoked`

const usedLinkQuery = `
UPDATE workspace_execution_preview_links
SET uses = uses + 1, last_used_at = $2, updated_at = now()
WHERE id = $1`

type shareRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.PreviewShare {
	return &shareRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *shareRepository) Create(
	ctx context.Context,
	link entity.PreviewShareLink,
) (entity.PreviewShareLink, error) {
	created, err := scanLink(r.db.Querier(ctx).QueryRowContext(
		ctx,
		createLinkQuery,
		link.PreviewID.String(),
		link.ExecutionID,
		link.WorkspaceID.String(),
		link.TokenHash,
		link.PasscodeHash,
		optionalID(link.CreatedBy),
		link.ExpiresAt,
	))
	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("create a preview share link: %w", err)
	}

	return created, nil
}

func (r *shareRepository) ByToken(
	ctx context.Context,
	tokenHash []byte,
) (entity.PreviewShareLink, error) {
	stored, err := scanLink(r.db.Querier(ctx).QueryRowContext(ctx, linkByTokenQuery, tokenHash))

	if errors.Is(err, sql.ErrNoRows) {
		return entity.PreviewShareLink{}, entity.ErrPreviewShareNotFound
	}

	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("read a preview share link: %w", err)
	}

	return stored, nil
}

func (r *shareRepository) ByPreview(
	ctx context.Context,
	previewID uuid.UUID,
) ([]entity.PreviewShareLink, error) {
	return r.list(ctx, linksByPreviewQuery, previewID.String())
}

func (r *shareRepository) ByExecution(
	ctx context.Context,
	executionID string,
) ([]entity.PreviewShareLink, error) {
	return r.list(ctx, linksByExecutionQuery, executionID)
}

func (r *shareRepository) list(
	ctx context.Context,
	query string,
	argument any,
) ([]entity.PreviewShareLink, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, argument)
	if err != nil {
		return nil, fmt.Errorf("list preview share links: %w", err)
	}

	defer func() { _ = rows.Close() }()

	links := make([]entity.PreviewShareLink, 0)

	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, fmt.Errorf("read a preview share link: %w", err)
		}

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read preview share links: %w", err)
	}

	return links, nil
}

func (r *shareRepository) Revoke(
	ctx context.Context,
	previewID, linkID uuid.UUID,
	revokedAt time.Time,
) (entity.PreviewShareLink, error) {
	revoked, err := scanLink(r.db.Querier(ctx).QueryRowContext(
		ctx, revokeLinkQuery, previewID.String(), linkID.String(), revokedAt,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return entity.PreviewShareLink{}, entity.ErrPreviewShareNotFound
	}

	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("revoke a preview share link: %w", err)
	}

	return revoked, nil
}

func (r *shareRepository) Used(ctx context.Context, linkID uuid.UUID, usedAt time.Time) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, usedLinkQuery, linkID.String(), usedAt,
	); err != nil {
		return fmt.Errorf("record the use of a preview share link: %w", err)
	}

	return nil
}

func scanLink(row scanner) (entity.PreviewShareLink, error) {
	var (
		link        entity.PreviewShareLink
		id          string
		previewID   string
		workspaceID string
		createdBy   string
		revokedAt   sql.NullTime
		lastUsedAt  sql.NullTime
	)

	if err := row.Scan(
		&id,
		&previewID,
		&link.ExecutionID,
		&workspaceID,
		&link.TokenHash,
		&link.PasscodeHash,
		&createdBy,
		&link.ExpiresAt,
		&revokedAt,
		&lastUsedAt,
		&link.Uses,
		&link.CreatedAt,
		&link.UpdatedAt,
	); err != nil {
		return entity.PreviewShareLink{}, err
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("parse preview share link id: %w", err)
	}

	parsedPreview, err := uuid.Parse(previewID)
	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("parse preview id: %w", err)
	}

	parsedWorkspace, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("parse workspace id: %w", err)
	}

	link.ID = parsedID
	link.PreviewID = parsedPreview
	link.WorkspaceID = parsedWorkspace

	creator, err := parseOptionalID(createdBy)
	if err != nil {
		return entity.PreviewShareLink{}, err
	}

	link.CreatedBy = creator

	if revokedAt.Valid {
		link.RevokedAt = revokedAt.Time
	}

	if lastUsedAt.Valid {
		link.LastUsedAt = lastUsedAt.Time
	}

	return link, nil
}

func optionalID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func parseOptionalID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, nil
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse account id: %w", err)
	}

	return parsed, nil
}

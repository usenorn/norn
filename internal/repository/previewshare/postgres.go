package previewshare

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
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

func linkOf(model *dbpostgres.WorkspaceExecutionPreviewLink) (entity.PreviewShareLink, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("parse preview share link id: %w", err)
	}

	previewID, err := uuid.Parse(model.PreviewID)
	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("parse preview id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("parse workspace id: %w", err)
	}

	createdBy, err := parseNullID(model.CreatedBy)
	if err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("parse preview share author id: %w", err)
	}

	link := entity.PreviewShareLink{
		ID:           id,
		PreviewID:    previewID,
		ExecutionID:  model.ExecutionID,
		WorkspaceID:  workspaceID,
		TokenHash:    model.TokenHash,
		PasscodeHash: model.PasscodeHash,
		CreatedBy:    createdBy,
		ExpiresAt:    model.ExpiresAt,
		Uses:         model.Uses,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}

	if model.RevokedAt.Valid {
		link.RevokedAt = model.RevokedAt.Time
	}

	if model.LastUsedAt.Valid {
		link.LastUsedAt = model.LastUsedAt.Time
	}

	return link, nil
}

func linksOf(
	models dbpostgres.WorkspaceExecutionPreviewLinkSlice,
) ([]entity.PreviewShareLink, error) {
	links := make([]entity.PreviewShareLink, 0, len(models))

	for _, model := range models {
		link, err := linkOf(model)
		if err != nil {
			return nil, err
		}

		links = append(links, link)
	}

	return links, nil
}

func nullID(id uuid.UUID) null.String {
	if id == uuid.Nil {
		return null.NewString("", false)
	}

	return null.StringFrom(id.String())
}

func parseNullID(value null.String) (uuid.UUID, error) {
	if !value.Valid || value.String == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(value.String)
}

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
	model := &dbpostgres.WorkspaceExecutionPreviewLink{
		PreviewID:    link.PreviewID.String(),
		ExecutionID:  link.ExecutionID,
		WorkspaceID:  link.WorkspaceID.String(),
		TokenHash:    link.TokenHash,
		PasscodeHash: link.PasscodeHash,
		CreatedBy:    nullID(link.CreatedBy),
		ExpiresAt:    link.ExpiresAt,
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return entity.PreviewShareLink{}, fmt.Errorf("create a preview share link: %w", err)
	}

	return linkOf(model)
}

func (r *shareRepository) ByToken(
	ctx context.Context,
	tokenHash []byte,
) (entity.PreviewShareLink, error) {
	model, err := dbpostgres.WorkspaceExecutionPreviewLinks(
		dbpostgres.WorkspaceExecutionPreviewLinkWhere.TokenHash.EQ(tokenHash),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.PreviewShareLink{}, entity.ErrPreviewShareNotFound
		}

		return entity.PreviewShareLink{}, fmt.Errorf("read a preview share link: %w", err)
	}

	return linkOf(model)
}

func (r *shareRepository) ByPreview(
	ctx context.Context,
	previewID uuid.UUID,
) ([]entity.PreviewShareLink, error) {
	return r.list(ctx, dbpostgres.WorkspaceExecutionPreviewLinkWhere.PreviewID.EQ(previewID.String()))
}

func (r *shareRepository) ByExecution(
	ctx context.Context,
	executionID string,
) ([]entity.PreviewShareLink, error) {
	return r.list(ctx, dbpostgres.WorkspaceExecutionPreviewLinkWhere.ExecutionID.EQ(executionID))
}

func (r *shareRepository) list(
	ctx context.Context,
	scope qm.QueryMod,
) ([]entity.PreviewShareLink, error) {
	models, err := dbpostgres.WorkspaceExecutionPreviewLinks(
		scope,
		qm.OrderBy(
			dbpostgres.WorkspaceExecutionPreviewLinkColumns.CreatedAt+", "+
				dbpostgres.WorkspaceExecutionPreviewLinkColumns.ID,
		),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list preview share links: %w", err)
	}

	return linksOf(models)
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

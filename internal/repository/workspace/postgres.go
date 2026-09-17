package workspace

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
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	slugUniqueIndex     = "workspaces_slug_key"
)

const (
	slugLockQuery = `SELECT pg_advisory_xact_lock(hashtext('workspace-slug:' || $1))`

	slugClaimedQuery = `
SELECT EXISTS (
    SELECT 1 FROM workspaces WHERE slug = $1 AND id <> $2
) OR EXISTS (
    SELECT 1 FROM workspace_slug_redirects WHERE slug = $1 AND workspace_id <> $2 AND expires_at > $3
)`

	releaseSlugRedirectQuery = `
DELETE FROM workspace_slug_redirects
WHERE slug = $1 AND (workspace_id = $2 OR expires_at <= $3)`

	recordSlugRedirectQuery = `
INSERT INTO workspace_slug_redirects (slug, workspace_id, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (slug) DO UPDATE
SET workspace_id = EXCLUDED.workspace_id, expires_at = EXCLUDED.expires_at, created_at = now()`

	resolveSlugRedirectQuery = `
SELECT workspace_id FROM workspace_slug_redirects WHERE slug = $1 AND expires_at > $2`
)

func toEntity(model *dbpostgres.Workspace) (entity.Workspace, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.Workspace{}, fmt.Errorf("parse workspace id: %w", err)
	}

	workspace := entity.Workspace{
		ID:            id,
		Slug:          model.Slug,
		Name:          model.Name,
		Status:        entity.WorkspaceStatus(model.Status),
		Timezone:      model.Timezone,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		WeekStartsOn:  entity.WeekDay(model.WeekStartsOn),
		LogoObjectKey: model.LogoObjectKey.String,
	}

	if model.DeletionRequestedAt.Valid {
		requestedAt := model.DeletionRequestedAt.Time
		workspace.DeletionRequestedAt = &requestedAt
	}

	if model.PurgeAfter.Valid {
		purgeAfter := model.PurgeAfter.Time
		workspace.PurgeAfter = &purgeAfter
	}

	if model.DefaultTeamID.Valid {
		defaultTeamID, err := uuid.Parse(model.DefaultTeamID.String)
		if err != nil {
			return entity.Workspace{}, fmt.Errorf("parse workspace default team id: %w", err)
		}

		workspace.DefaultTeamID = &defaultTeamID
	}

	return workspace, nil
}

func toModel(workspace entity.Workspace) *dbpostgres.Workspace {
	model := &dbpostgres.Workspace{
		ID:           workspace.ID.String(),
		Slug:         workspace.Slug,
		Name:         workspace.Name,
		Status:       string(workspace.Status),
		Timezone:     workspace.Timezone,
		CreatedAt:    workspace.CreatedAt,
		UpdatedAt:    workspace.UpdatedAt,
		WeekStartsOn: string(workspace.WeekStartsOn),
	}

	if workspace.LogoObjectKey != "" {
		model.LogoObjectKey = null.StringFrom(workspace.LogoObjectKey)
	}

	if workspace.DeletionRequestedAt != nil {
		model.DeletionRequestedAt = null.TimeFrom(*workspace.DeletionRequestedAt)
	}

	if workspace.PurgeAfter != nil {
		model.PurgeAfter = null.TimeFrom(*workspace.PurgeAfter)
	}

	if workspace.DefaultTeamID != nil {
		model.DefaultTeamID = null.StringFrom(workspace.DefaultTeamID.String())
	}

	return model
}

type workspaceRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Workspace {
	return &workspaceRepository{db: db}
}

func (r *workspaceRepository) Create(ctx context.Context, workspace entity.Workspace) (entity.Workspace, error) {
	if workspace.ID == uuid.Nil {
		workspace.ID = uuid.New()
	}

	now := time.Now().UTC()
	workspace.CreatedAt = now
	workspace.UpdatedAt = now

	model := toModel(workspace)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		if slugTaken(err) {
			return entity.Workspace{}, entity.ErrWorkspaceSlugTaken
		}

		return entity.Workspace{}, fmt.Errorf("insert workspace: %w", err)
	}

	return toEntity(model)
}

func (r *workspaceRepository) GetByID(ctx context.Context, id uuid.UUID) (entity.Workspace, error) {
	model, err := dbpostgres.FindWorkspace(ctx, r.db.Querier(ctx), id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Workspace{}, entity.ErrWorkspaceNotFound
		}

		return entity.Workspace{}, fmt.Errorf("find workspace by id: %w", err)
	}

	return toEntity(model)
}

func (r *workspaceRepository) GetBySlug(ctx context.Context, slug string) (entity.Workspace, error) {
	model, err := dbpostgres.Workspaces(
		dbpostgres.WorkspaceWhere.Slug.EQ(slug),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Workspace{}, entity.ErrWorkspaceNotFound
		}

		return entity.Workspace{}, fmt.Errorf("find workspace by slug: %w", err)
	}

	return toEntity(model)
}

func (r *workspaceRepository) UpdateSettings(
	ctx context.Context,
	id uuid.UUID,
	settings repository.WorkspaceSettings,
) (entity.Workspace, error) {
	defaultTeam := null.NewString("", false)
	if settings.DefaultTeamID != nil {
		defaultTeam = null.StringFrom(settings.DefaultTeamID.String())
	}

	updated, err := dbpostgres.Workspaces(
		dbpostgres.WorkspaceWhere.ID.EQ(id.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceColumns.Slug:          settings.Slug,
		dbpostgres.WorkspaceColumns.Name:          settings.Name,
		dbpostgres.WorkspaceColumns.Timezone:      settings.Timezone,
		dbpostgres.WorkspaceColumns.WeekStartsOn:  string(settings.WeekStartsOn),
		dbpostgres.WorkspaceColumns.DefaultTeamID: defaultTeam,
		dbpostgres.WorkspaceColumns.UpdatedAt:     time.Now().UTC(),
	})
	if err != nil {
		if slugTaken(err) {
			return entity.Workspace{}, entity.ErrWorkspaceSlugTaken
		}

		return entity.Workspace{}, fmt.Errorf("update workspace settings: %w", err)
	}

	if updated == 0 {
		return entity.Workspace{}, entity.ErrWorkspaceNotFound
	}

	return r.GetByID(ctx, id)
}

func (r *workspaceRepository) SetLogo(ctx context.Context, id uuid.UUID, objectKey string) (entity.Workspace, error) {
	logo := null.NewString("", false)
	if objectKey != "" {
		logo = null.StringFrom(objectKey)
	}

	updated, err := dbpostgres.Workspaces(
		dbpostgres.WorkspaceWhere.ID.EQ(id.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceColumns.LogoObjectKey: logo,
		dbpostgres.WorkspaceColumns.UpdatedAt:     time.Now().UTC(),
	})
	if err != nil {
		return entity.Workspace{}, fmt.Errorf("set workspace logo: %w", err)
	}

	if updated == 0 {
		return entity.Workspace{}, entity.ErrWorkspaceNotFound
	}

	return r.GetByID(ctx, id)
}

func (r *workspaceRepository) ReserveSlug(ctx context.Context, slug string, claimant uuid.UUID, now time.Time) error {
	querier := r.db.Querier(ctx)

	if _, err := querier.ExecContext(ctx, slugLockQuery, slug); err != nil {
		return fmt.Errorf("lock workspace slug: %w", err)
	}

	var claimed bool

	if err := querier.QueryRowContext(ctx, slugClaimedQuery, slug, claimant.String(), now).Scan(&claimed); err != nil {
		return fmt.Errorf("check workspace slug: %w", err)
	}

	if claimed {
		return entity.ErrWorkspaceSlugTaken
	}

	if _, err := querier.ExecContext(ctx, releaseSlugRedirectQuery, slug, claimant.String(), now); err != nil {
		return fmt.Errorf("release workspace slug redirect: %w", err)
	}

	return nil
}

func (r *workspaceRepository) RecordSlugRedirect(
	ctx context.Context,
	slug string,
	workspaceID uuid.UUID,
	expiresAt time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, recordSlugRedirectQuery, slug, workspaceID.String(), expiresAt); err != nil {
		return fmt.Errorf("record workspace slug redirect: %w", err)
	}

	return nil
}

func (r *workspaceRepository) ResolveSlugRedirect(ctx context.Context, slug string, now time.Time) (uuid.UUID, error) {
	var workspaceID uuid.UUID

	if err := r.db.Querier(ctx).QueryRowContext(ctx, resolveSlugRedirectQuery, slug, now).Scan(&workspaceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, entity.ErrWorkspaceNotFound
		}

		return uuid.Nil, fmt.Errorf("resolve workspace slug redirect: %w", err)
	}

	return workspaceID, nil
}

func (r *workspaceRepository) MarkPendingDeletion(
	ctx context.Context,
	id uuid.UUID,
	requestedAt, purgeAfter time.Time,
) (entity.Workspace, error) {
	updated, err := dbpostgres.Workspaces(
		dbpostgres.WorkspaceWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceWhere.Status.EQ(string(entity.WorkspaceStatusActive)),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceColumns.Status:              string(entity.WorkspaceStatusPendingDeletion),
		dbpostgres.WorkspaceColumns.DeletionRequestedAt: null.TimeFrom(requestedAt),
		dbpostgres.WorkspaceColumns.PurgeAfter:          null.TimeFrom(purgeAfter),
		dbpostgres.WorkspaceColumns.UpdatedAt:           time.Now().UTC(),
	})
	if err != nil {
		return entity.Workspace{}, fmt.Errorf("mark workspace pending deletion: %w", err)
	}

	if updated == 0 {
		return entity.Workspace{}, entity.ErrWorkspaceDeleted
	}

	return r.GetByID(ctx, id)
}

func (r *workspaceRepository) Restore(ctx context.Context, id uuid.UUID) (entity.Workspace, error) {
	updated, err := dbpostgres.Workspaces(
		dbpostgres.WorkspaceWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceWhere.Status.EQ(string(entity.WorkspaceStatusPendingDeletion)),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceColumns.Status:              string(entity.WorkspaceStatusActive),
		dbpostgres.WorkspaceColumns.DeletionRequestedAt: null.NewTime(time.Time{}, false),
		dbpostgres.WorkspaceColumns.PurgeAfter:          null.NewTime(time.Time{}, false),
		dbpostgres.WorkspaceColumns.UpdatedAt:           time.Now().UTC(),
	})
	if err != nil {
		return entity.Workspace{}, fmt.Errorf("restore workspace: %w", err)
	}

	if updated == 0 {
		return entity.Workspace{}, entity.ErrWorkspaceNotDeleted
	}

	return r.GetByID(ctx, id)
}

func (r *workspaceRepository) Purge(ctx context.Context, id uuid.UUID) error {
	if _, err := dbpostgres.Workspaces(
		dbpostgres.WorkspaceWhere.ID.EQ(id.String()),
	).DeleteAll(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("purge workspace: %w", err)
	}

	return nil
}

func (r *workspaceRepository) ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]entity.Workspace, error) {
	models, err := dbpostgres.Workspaces(
		qm.InnerJoin("workspace_memberships m on m.workspace_id = workspaces.id"),
		qm.Where("m.account_id = ?", accountID.String()),
		qm.OrderBy("workspaces.created_at"),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list workspaces by account: %w", err)
	}

	workspaces := make([]entity.Workspace, 0, len(models))

	for _, model := range models {
		workspace, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		workspaces = append(workspaces, workspace)
	}

	return workspaces, nil
}

func (r *workspaceRepository) LockByIDs(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = id.String()
	}

	if _, err := dbpostgres.Workspaces(
		dbpostgres.WorkspaceWhere.ID.IN(keys),
		qm.OrderBy(dbpostgres.WorkspaceColumns.ID),
		qm.For("UPDATE"),
	).All(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("lock workspaces: %w", err)
	}

	return nil
}

func slugTaken(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == slugUniqueIndex
}

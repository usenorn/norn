package savedview

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type savedViewRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.SavedView {
	return &savedViewRepository{db: db}
}

const savedViewColumns = `
       v.id,
       v.workspace_id,
       coalesce(v.created_by_account_id::text, ''),
       coalesce(a.display_name, ''),
       v.sharing,
       coalesce(v.team_id::text, ''),
       coalesce(t.name, ''),
       v.name,
       v.filter,
       v.sort,
       v.group_by,
       v.created_at,
       v.updated_at`

const savedViewJoins = `
FROM workspace_saved_views v
LEFT JOIN workspace_teams t ON t.id = v.team_id
LEFT JOIN accounts a ON a.id = v.created_by_account_id`

const savedViewByIDQuery = `SELECT` + savedViewColumns + savedViewJoins + `
WHERE v.id = $1 AND v.workspace_id = $2`

const lockSavedViewQuery = `SELECT` + savedViewColumns + savedViewJoins + `
WHERE v.id = $1 AND v.workspace_id = $2
FOR UPDATE OF v`

const savedViewsForAccountQuery = `SELECT` + savedViewColumns + savedViewJoins + `
LEFT JOIN workspace_saved_view_placements p
       ON p.saved_view_id = v.id AND p.account_id = $2::uuid
WHERE v.workspace_id = $1
  AND (
        v.sharing = 'workspace'
     OR (v.sharing = 'personal' AND v.created_by_account_id = $2::uuid)
     OR (v.sharing = 'team' AND v.team_id = ANY($3::uuid[]))
  )
ORDER BY p.position NULLS LAST, v.created_at, v.id`

const createSavedViewQuery = `
WITH created AS (
    INSERT INTO workspace_saved_views
        (workspace_id, created_by_account_id, sharing, team_id, name, filter, sort, group_by)
    VALUES ($1, nullif($2, '')::uuid, $3, nullif($4, '')::uuid, $5, $6::jsonb, $7::jsonb, $8)
    RETURNING *
)
SELECT` + savedViewColumns + `
FROM created v
LEFT JOIN workspace_teams t ON t.id = v.team_id
LEFT JOIN accounts a ON a.id = v.created_by_account_id`

const updateSavedViewQuery = `
WITH updated AS (
    UPDATE workspace_saved_views
    SET name = $2,
        sharing = $3,
        team_id = nullif($4, '')::uuid,
        filter = $5::jsonb,
        sort = $6::jsonb,
        group_by = $7,
        updated_at = $8
    WHERE id = $1
    RETURNING *
)
SELECT` + savedViewColumns + `
FROM updated v
LEFT JOIN workspace_teams t ON t.id = v.team_id
LEFT JOIN accounts a ON a.id = v.created_by_account_id`

const deleteSavedViewQuery = `DELETE FROM workspace_saved_views WHERE id = $1`

const clearPlacementsQuery = `
DELETE FROM workspace_saved_view_placements
WHERE workspace_id = $1 AND account_id = $2`

const placeSavedViewsQuery = `
INSERT INTO workspace_saved_view_placements
    (workspace_id, account_id, saved_view_id, position, created_at, updated_at)
SELECT $1, $2, ordered.id, ordered.position, $4, $4
FROM (
    SELECT id, ordinality AS position
    FROM unnest($3::uuid[]) WITH ORDINALITY AS t (id, ordinality)
) AS ordered`

type scanner interface {
	Scan(dest ...any) error
}

func scanSavedView(row scanner) (entity.SavedView, error) {
	var (
		view               entity.SavedView
		id, workspace      string
		author, team       string
		sharing, groupBy   string
		filterRaw, sortRaw []byte
	)

	if err := row.Scan(
		&id, &workspace, &author, &view.AuthorName, &sharing, &team, &view.TeamName,
		&view.Name, &filterRaw, &sortRaw, &groupBy, &view.CreatedAt, &view.UpdatedAt,
	); err != nil {
		return entity.SavedView{}, err
	}

	view.Sharing = entity.SavedViewSharing(sharing)
	view.GroupBy = entity.IssueGroupBy(groupBy)

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.SavedView{}, fmt.Errorf("parse saved view id: %w", err)
	}

	view.ID = parsed

	if view.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.SavedView{}, fmt.Errorf("parse saved view workspace id: %w", err)
	}

	if author != "" {
		if view.AuthorID, err = uuid.Parse(author); err != nil {
			return entity.SavedView{}, fmt.Errorf("parse saved view author id: %w", err)
		}
	}

	if team != "" {
		if view.TeamID, err = uuid.Parse(team); err != nil {
			return entity.SavedView{}, fmt.Errorf("parse saved view team id: %w", err)
		}
	}

	if err := json.Unmarshal(filterRaw, &view.Filter); err != nil {
		return entity.SavedView{}, fmt.Errorf("decode saved view filter: %w", err)
	}

	if err := json.Unmarshal(sortRaw, &view.Sort); err != nil {
		return entity.SavedView{}, fmt.Errorf("decode saved view sort: %w", err)
	}

	return view, nil
}

func encode(filter entity.IssueFilter, sort []entity.IssueSort) (string, string, error) {
	filterRaw, err := json.Marshal(filter)
	if err != nil {
		return "", "", fmt.Errorf("encode saved view filter: %w", err)
	}

	if sort == nil {
		sort = []entity.IssueSort{}
	}

	sortRaw, err := json.Marshal(sort)
	if err != nil {
		return "", "", fmt.Errorf("encode saved view sort: %w", err)
	}

	return string(filterRaw), string(sortRaw), nil
}

func text(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func (r *savedViewRepository) find(
	ctx context.Context,
	query string,
	args ...any,
) (entity.SavedView, error) {
	view, err := scanSavedView(r.db.Querier(ctx).QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.SavedView{}, entity.ErrSavedViewNotFound
		}

		return entity.SavedView{}, fmt.Errorf("find saved view: %w", err)
	}

	return view, nil
}

func (r *savedViewRepository) Create(
	ctx context.Context,
	view entity.SavedView,
) (entity.SavedView, error) {
	filterRaw, sortRaw, err := encode(view.Filter, view.Sort)
	if err != nil {
		return entity.SavedView{}, err
	}

	created, err := scanSavedView(r.db.Querier(ctx).QueryRowContext(
		ctx, createSavedViewQuery,
		view.WorkspaceID.String(), text(view.AuthorID), string(view.Sharing),
		text(view.TeamID), view.Name, filterRaw, sortRaw, string(view.GroupBy),
	))
	if err != nil {
		return entity.SavedView{}, fmt.Errorf("create saved view: %w", err)
	}

	return created, nil
}

func (r *savedViewRepository) GetByID(
	ctx context.Context,
	workspaceID, savedViewID uuid.UUID,
) (entity.SavedView, error) {
	return r.find(ctx, savedViewByIDQuery, savedViewID.String(), workspaceID.String())
}

func (r *savedViewRepository) LockByID(
	ctx context.Context,
	workspaceID, savedViewID uuid.UUID,
) (entity.SavedView, error) {
	return r.find(ctx, lockSavedViewQuery, savedViewID.String(), workspaceID.String())
}

func (r *savedViewRepository) ListFor(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	teamIDs []uuid.UUID,
) ([]entity.SavedView, error) {
	teams := make([]string, 0, len(teamIDs))
	for _, id := range teamIDs {
		teams = append(teams, id.String())
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, savedViewsForAccountQuery, workspaceID.String(), accountID.String(), teams,
	)
	if err != nil {
		return nil, fmt.Errorf("list saved views: %w", err)
	}

	defer func() { _ = rows.Close() }()

	views := make([]entity.SavedView, 0)

	for rows.Next() {
		view, err := scanSavedView(rows)
		if err != nil {
			return nil, fmt.Errorf("scan saved view: %w", err)
		}

		views = append(views, view)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read saved views: %w", err)
	}

	return views, nil
}

func (r *savedViewRepository) UpdateSettings(
	ctx context.Context,
	savedViewID uuid.UUID,
	settings repository.SavedViewSettings,
) (entity.SavedView, error) {
	filterRaw, sortRaw, err := encode(settings.Filter, settings.Sort)
	if err != nil {
		return entity.SavedView{}, err
	}

	updated, err := scanSavedView(r.db.Querier(ctx).QueryRowContext(
		ctx, updateSavedViewQuery,
		savedViewID.String(), settings.Name, string(settings.Sharing), text(settings.TeamID),
		filterRaw, sortRaw, string(settings.GroupBy), time.Now().UTC(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.SavedView{}, entity.ErrSavedViewNotFound
		}

		return entity.SavedView{}, fmt.Errorf("update saved view: %w", err)
	}

	return updated, nil
}

func (r *savedViewRepository) Delete(ctx context.Context, savedViewID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteSavedViewQuery, savedViewID.String())
	if err != nil {
		return fmt.Errorf("delete saved view: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete saved view: %w", err)
	}

	if affected == 0 {
		return entity.ErrSavedViewNotFound
	}

	return nil
}

func (r *savedViewRepository) Place(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	orderedIDs []uuid.UUID,
) error {
	querier := r.db.Querier(ctx)

	if _, err := querier.ExecContext(
		ctx, clearPlacementsQuery, workspaceID.String(), accountID.String(),
	); err != nil {
		return fmt.Errorf("clear saved view placements: %w", err)
	}

	if len(orderedIDs) == 0 {
		return nil
	}

	ids := make([]string, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		ids = append(ids, id.String())
	}

	if _, err := querier.ExecContext(
		ctx, placeSavedViewsQuery, workspaceID.String(), accountID.String(), ids, time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("place saved views: %w", err)
	}

	return nil
}

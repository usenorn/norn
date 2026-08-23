package preview

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

const previewColumns = `
       id,
       execution_id,
       workspace_id,
       name,
       service,
       path,
       mode,
       host,
       state,
       opened_at,
       closed_at,
       reported_at,
       created_at,
       updated_at`

const savePreviewQuery = `
WITH upserted AS (
    INSERT INTO workspace_execution_previews
        (execution_id, workspace_id, name, service, path, mode, host, state,
         opened_at, closed_at, reported_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    ON CONFLICT (execution_id, name) DO UPDATE
    SET service     = excluded.service,
        path        = excluded.path,
        mode        = excluded.mode,
        host        = excluded.host,
        state       = excluded.state,
        opened_at   = CASE WHEN excluded.state = 'open'
                            AND workspace_execution_previews.state = 'closed'
                           THEN excluded.opened_at
                           ELSE workspace_execution_previews.opened_at END,
        closed_at   = CASE WHEN excluded.state = 'closed'
                           THEN coalesce(excluded.closed_at, now())
                           ELSE NULL END,
        reported_at = excluded.reported_at,
        updated_at  = now()
    WHERE excluded.reported_at >= workspace_execution_previews.reported_at
    RETURNING *
)
SELECT` + previewColumns + `
FROM upserted`

const previewByNameQuery = `
SELECT` + previewColumns + `
FROM workspace_execution_previews
WHERE execution_id = $1 AND name = $2`

const previewByHostQuery = `
SELECT` + previewColumns + `
FROM workspace_execution_previews
WHERE host = $1 AND host <> ''`

const previewRouteByHostQuery = `
SELECT p.id,
       p.execution_id,
       p.workspace_id,
       p.name,
       p.service,
       p.path,
       p.mode,
       p.host,
       p.state,
       p.opened_at,
       p.closed_at,
       p.reported_at,
       p.created_at,
       p.updated_at,
       coalesce(e.runner_id, '00000000-0000-0000-0000-000000000000'::uuid)
FROM workspace_execution_previews p
         JOIN workspace_executions e ON e.id = p.execution_id
WHERE p.host = $1 AND p.host <> ''`

const previewsQuery = `
SELECT` + previewColumns + `
FROM workspace_execution_previews
WHERE execution_id = $1
ORDER BY name, id`

const previewCountQuery = `
SELECT count(*)
FROM workspace_execution_previews
WHERE execution_id = $1`

const closePreviewsQuery = `
UPDATE workspace_execution_previews
SET state = 'closed', closed_at = coalesce(closed_at, $2), updated_at = now()
WHERE execution_id = $1 AND state = 'open'`

type previewRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Preview {
	return &previewRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *previewRepository) Save(
	ctx context.Context,
	preview entity.PreviewSession,
) (entity.PreviewSession, error) {
	saved, err := scanPreview(r.db.Querier(ctx).QueryRowContext(
		ctx,
		savePreviewQuery,
		preview.ExecutionID,
		preview.WorkspaceID.String(),
		preview.Name,
		preview.Service,
		preview.Path,
		string(preview.Mode),
		preview.Host,
		string(preview.State),
		preview.OpenedAt,
		optionalTime(preview.ClosedAt),
		preview.ReportedAt,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return r.ByName(ctx, preview.ExecutionID, preview.Name)
	}

	if err != nil {
		return entity.PreviewSession{}, fmt.Errorf("save preview: %w", err)
	}

	return saved, nil
}

func (r *previewRepository) ByName(
	ctx context.Context,
	executionID, name string,
) (entity.PreviewSession, error) {
	stored, err := scanPreview(r.db.Querier(ctx).QueryRowContext(
		ctx, previewByNameQuery, executionID, name,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return entity.PreviewSession{}, entity.ErrPreviewNotFound
	}

	if err != nil {
		return entity.PreviewSession{}, fmt.Errorf("read preview: %w", err)
	}

	return stored, nil
}

func (r *previewRepository) ByHost(
	ctx context.Context,
	host string,
) (entity.PreviewSession, error) {
	stored, err := scanPreview(r.db.Querier(ctx).QueryRowContext(ctx, previewByHostQuery, host))

	if errors.Is(err, sql.ErrNoRows) {
		return entity.PreviewSession{}, entity.ErrPreviewNotFound
	}

	if err != nil {
		return entity.PreviewSession{}, fmt.Errorf("read preview by host: %w", err)
	}

	return stored, nil
}

func (r *previewRepository) RouteByHost(
	ctx context.Context,
	host string,
) (entity.PreviewRoute, error) {
	route, err := scanRoute(r.db.Querier(ctx).QueryRowContext(ctx, previewRouteByHostQuery, host))

	if errors.Is(err, sql.ErrNoRows) {
		return entity.PreviewRoute{}, entity.ErrPreviewNotFound
	}

	if err != nil {
		return entity.PreviewRoute{}, fmt.Errorf("read preview route by host: %w", err)
	}

	return route, nil
}

func (r *previewRepository) ByExecution(
	ctx context.Context,
	executionID string,
) ([]entity.PreviewSession, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, previewsQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("list previews: %w", err)
	}

	defer func() { _ = rows.Close() }()

	previews := make([]entity.PreviewSession, 0)

	for rows.Next() {
		preview, err := scanPreview(rows)
		if err != nil {
			return nil, fmt.Errorf("read a preview: %w", err)
		}

		previews = append(previews, preview)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read previews: %w", err)
	}

	return previews, nil
}

func (r *previewRepository) Count(ctx context.Context, executionID string) (int, error) {
	var held int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, previewCountQuery, executionID,
	).Scan(&held); err != nil {
		return 0, fmt.Errorf("count previews: %w", err)
	}

	return held, nil
}

func (r *previewRepository) CloseByExecution(
	ctx context.Context,
	executionID string,
	closedAt time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, closePreviewsQuery, executionID, closedAt,
	); err != nil {
		return fmt.Errorf("close the previews of a run: %w", err)
	}

	return nil
}

func scanRoute(row scanner) (entity.PreviewRoute, error) {
	var (
		route    entity.PreviewRoute
		runnerID string
	)

	preview, err := scanPreviewWith(row, &runnerID)
	if err != nil {
		return entity.PreviewRoute{}, err
	}

	parsedRunner, err := uuid.Parse(runnerID)
	if err != nil {
		return entity.PreviewRoute{}, fmt.Errorf("parse runner id: %w", err)
	}

	route.Preview = preview
	route.RunnerID = parsedRunner

	return route, nil
}

func scanPreview(row scanner) (entity.PreviewSession, error) {
	return scanPreviewWith(row)
}

func scanPreviewWith(row scanner, extra ...any) (entity.PreviewSession, error) {
	var (
		preview     entity.PreviewSession
		id          string
		workspaceID string
		mode        string
		state       string
		closedAt    sql.NullTime
	)

	into := []any{
		&id,
		&preview.ExecutionID,
		&workspaceID,
		&preview.Name,
		&preview.Service,
		&preview.Path,
		&mode,
		&preview.Host,
		&state,
		&preview.OpenedAt,
		&closedAt,
		&preview.ReportedAt,
		&preview.CreatedAt,
		&preview.UpdatedAt,
	}

	if err := row.Scan(append(into, extra...)...); err != nil {
		return entity.PreviewSession{}, err
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return entity.PreviewSession{}, fmt.Errorf("parse preview id: %w", err)
	}

	parsedWorkspace, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.PreviewSession{}, fmt.Errorf("parse workspace id: %w", err)
	}

	preview.ID = parsedID
	preview.WorkspaceID = parsedWorkspace
	preview.Mode = entity.PreviewMode(mode)
	preview.State = entity.PreviewState(state)

	if closedAt.Valid {
		preview.ClosedAt = closedAt.Time
	}

	return preview, nil
}

func optionalTime(at time.Time) any {
	if at.IsZero() {
		return nil
	}

	return at
}

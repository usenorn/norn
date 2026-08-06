package imports

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const cursorsByRunQuery = `
SELECT run_id, resource, cursor, exhausted, fetched, retry_after, updated_at
FROM workspace_import_cursors
WHERE run_id = $1
ORDER BY resource`

const saveCursorQuery = `
INSERT INTO workspace_import_cursors (
    run_id, resource, cursor, exhausted, fetched, retry_after, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (run_id, resource) DO UPDATE SET
    cursor = excluded.cursor,
    exhausted = excluded.exhausted,
    fetched = excluded.fetched,
    retry_after = excluded.retry_after,
    updated_at = excluded.updated_at`

const parkCursorQuery = `
INSERT INTO workspace_import_cursors (run_id, resource, retry_after, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (run_id, resource) DO UPDATE SET
    retry_after = excluded.retry_after,
    updated_at = excluded.updated_at`

type cursorRepository struct {
	db *postgres.Client
}

func NewImportCursor(db *postgres.Client) repository.ImportCursor {
	return &cursorRepository{db: db}
}

func (r *cursorRepository) List(
	ctx context.Context,
	runID uuid.UUID,
) ([]entity.ImportCursor, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, cursorsByRunQuery, runID.String())
	if err != nil {
		return nil, fmt.Errorf("list import cursors: %w", err)
	}

	defer func() { _ = rows.Close() }()

	cursors := make([]entity.ImportCursor, 0, len(entity.ImportPhases()))

	for rows.Next() {
		var (
			cursor     entity.ImportCursor
			rawRun     string
			resource   string
			retryAfter sql.NullTime
		)

		if err := rows.Scan(
			&rawRun, &resource, &cursor.Cursor, &cursor.Exhausted,
			&cursor.Fetched, &retryAfter, &cursor.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan import cursor: %w", err)
		}

		cursor.RunID = parseID(rawRun)
		cursor.Resource = entity.ImportResource(resource)
		cursor.RetryAfter = optionalTime(retryAfter)

		cursors = append(cursors, cursor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read import cursors: %w", err)
	}

	return cursors, nil
}

func (r *cursorRepository) Save(ctx context.Context, cursor entity.ImportCursor) error {
	if cursor.UpdatedAt.IsZero() {
		cursor.UpdatedAt = time.Now().UTC()
	}

	var retryAfter sql.NullTime

	if cursor.RetryAfter != nil {
		retryAfter = sql.NullTime{Time: *cursor.RetryAfter, Valid: true}
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		saveCursorQuery,
		cursor.RunID.String(),
		string(cursor.Resource),
		cursor.Cursor,
		cursor.Exhausted,
		cursor.Fetched,
		retryAfter,
		cursor.UpdatedAt,
	); err != nil {
		return fmt.Errorf("save import cursor: %w", err)
	}

	return nil
}

func (r *cursorRepository) Park(
	ctx context.Context,
	runID uuid.UUID,
	resource entity.ImportResource,
	until time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		parkCursorQuery,
		runID.String(),
		string(resource),
		until,
		time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("park import cursor: %w", err)
	}

	return nil
}

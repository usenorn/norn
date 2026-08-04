package project

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const statusColumns = `
       s.id,
       s.project_id,
       coalesce(s.author_account_id::text, ''),
       coalesce(a.display_name, ''),
       s.health,
       s.body,
       s.created_at`

const insertStatusQuery = `
WITH inserted AS (
    INSERT INTO workspace_project_status_updates (project_id, author_account_id, health, body)
    VALUES ($1, $2::uuid, $3, $4)
    RETURNING id, project_id, author_account_id, health, body, created_at
)
SELECT` + statusColumns + `
FROM inserted s
LEFT JOIN accounts a ON a.id = s.author_account_id`

const statusUpdatesQuery = `
SELECT` + statusColumns + `
FROM workspace_project_status_updates s
LEFT JOIN accounts a ON a.id = s.author_account_id
WHERE s.project_id = $1
ORDER BY s.created_at DESC, s.id DESC`

type statusRepository struct {
	db *postgres.Client
}

func NewStatusUpdate(db *postgres.Client) repository.ProjectStatusUpdate {
	return &statusRepository{db: db}
}

func scanStatusUpdate(row scanner) (entity.ProjectStatusUpdate, error) {
	var (
		update  entity.ProjectStatusUpdate
		id      string
		project string
		author  string
		health  string
	)

	if err := row.Scan(
		&id,
		&project,
		&author,
		&update.AuthorName,
		&health,
		&update.Body,
		&update.CreatedAt,
	); err != nil {
		return entity.ProjectStatusUpdate{}, err
	}

	update.Health = entity.ProjectHealth(health)

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.ProjectStatusUpdate{}, fmt.Errorf("parse status update id: %w", err)
	}

	update.ID = parsed

	if update.ProjectID, err = uuid.Parse(project); err != nil {
		return entity.ProjectStatusUpdate{}, fmt.Errorf("parse status update project id: %w", err)
	}

	if author != "" {
		if update.AuthorAccountID, err = uuid.Parse(author); err != nil {
			return entity.ProjectStatusUpdate{}, fmt.Errorf("parse status update author id: %w", err)
		}
	}

	return update, nil
}

func (r *statusRepository) Record(
	ctx context.Context,
	update entity.ProjectStatusUpdate,
) (entity.ProjectStatusUpdate, error) {
	var author any

	if update.AuthorAccountID != uuid.Nil {
		author = update.AuthorAccountID.String()
	}

	recorded, err := scanStatusUpdate(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertStatusQuery,
		update.ProjectID.String(),
		author,
		string(update.Health),
		update.Body,
	))
	if err != nil {
		return entity.ProjectStatusUpdate{}, fmt.Errorf("record project status update: %w", err)
	}

	return recorded, nil
}

func (r *statusRepository) ListByProjectID(
	ctx context.Context,
	projectID uuid.UUID,
) ([]entity.ProjectStatusUpdate, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, statusUpdatesQuery, projectID.String())
	if err != nil {
		return nil, fmt.Errorf("list project status updates: %w", err)
	}

	defer func() { _ = rows.Close() }()

	updates := make([]entity.ProjectStatusUpdate, 0)

	for rows.Next() {
		update, err := scanStatusUpdate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project status update: %w", err)
		}

		updates = append(updates, update)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project status updates: %w", err)
	}

	return updates, nil
}

package project

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const linkColumns = `
       l.id,
       l.project_id,
       l.label,
       l.url,
       l.created_at,
       l.updated_at`

const insertLinkQuery = `
WITH placed AS (
    SELECT coalesce(max(position) + 1, 0) AS next
    FROM workspace_project_links
    WHERE project_id = $1
)
INSERT INTO workspace_project_links (project_id, label, url, position)
SELECT $1, $2, $3, placed.next FROM placed
RETURNING id, project_id, label, url, created_at, updated_at`

const linksQuery = `
SELECT` + linkColumns + `
FROM workspace_project_links l
WHERE l.project_id = $1
ORDER BY l.position, l.id`

const deleteLinkQuery = `DELETE FROM workspace_project_links WHERE id = $1 AND project_id = $2`

type linkRepository struct {
	db *postgres.Client
}

func NewLink(db *postgres.Client) repository.ProjectLink {
	return &linkRepository{db: db}
}

func scanLink(row scanner) (entity.ProjectLink, error) {
	var (
		link    entity.ProjectLink
		id      string
		project string
	)

	if err := row.Scan(
		&id,
		&project,
		&link.Label,
		&link.URL,
		&link.CreatedAt,
		&link.UpdatedAt,
	); err != nil {
		return entity.ProjectLink{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.ProjectLink{}, fmt.Errorf("parse project link id: %w", err)
	}

	link.ID = parsed

	if link.ProjectID, err = uuid.Parse(project); err != nil {
		return entity.ProjectLink{}, fmt.Errorf("parse project link project id: %w", err)
	}

	return link, nil
}

func (r *linkRepository) Add(
	ctx context.Context,
	projectID uuid.UUID,
	label, address string,
) (entity.ProjectLink, error) {
	added, err := scanLink(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertLinkQuery,
		projectID.String(),
		label,
		address,
	))
	if err != nil {
		return entity.ProjectLink{}, fmt.Errorf("add project link: %w", err)
	}

	return added, nil
}

func (r *linkRepository) ListByProjectID(
	ctx context.Context,
	projectID uuid.UUID,
) ([]entity.ProjectLink, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, linksQuery, projectID.String())
	if err != nil {
		return nil, fmt.Errorf("list project links: %w", err)
	}

	defer func() { _ = rows.Close() }()

	links := make([]entity.ProjectLink, 0)

	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project link: %w", err)
		}

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project links: %w", err)
	}

	return links, nil
}

func (r *linkRepository) Remove(ctx context.Context, projectID, linkID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		deleteLinkQuery,
		linkID.String(),
		projectID.String(),
	)
	if err != nil {
		return fmt.Errorf("remove project link: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove project link: %w", err)
	}

	if removed == 0 {
		return entity.ErrProjectLinkNotFound
	}

	return nil
}

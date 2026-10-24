package doclink

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const uniqueViolationCode = "23505"

var uniqueLinkIndexes = []string{"workspace_doc_links_issue_key", "workspace_doc_links_project_key"}

const linksForDocQuery = `
SELECT l.id,
       l.doc_id,
       l.created_at,
       coalesce(o.id::text, ''),
       coalesce(o.reference_key, ''),
       coalesce(o.number, 0),
       coalesce(o.title, ''),
       coalesce(o.status, ''),
       coalesce(o.team_id::text, ''),
       coalesce(t.key, ''),
       coalesce(s.id::text, ''),
       coalesce(s.name, ''),
       coalesce(s.category, ''),
       coalesce(s.position, 0),
       coalesce(p.id::text, ''),
       coalesce(p.slug, ''),
       coalesce(p.name, ''),
       p.archived_at
FROM workspace_doc_links l
LEFT JOIN workspace_issues o ON o.id = l.issue_id
LEFT JOIN workspace_teams t ON t.id = o.team_id
LEFT JOIN workspace_workflow_states s ON s.id = o.state_id AND s.team_id = o.team_id
LEFT JOIN workspace_projects p ON p.id = l.project_id
WHERE l.workspace_id = $1
  AND l.doc_id = $2
  AND (l.issue_id IS NULL OR $3::boolean IS TRUE OR o.team_id = ANY($4::uuid[]))
ORDER BY l.created_at, l.id`

type linkRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.DocLink {
	return &linkRepository{db: db}
}

func linkToModel(link entity.StoredDocLink) *dbpostgres.WorkspaceDocLink {
	return &dbpostgres.WorkspaceDocLink{
		ID:                 link.ID.String(),
		WorkspaceID:        link.WorkspaceID.String(),
		DocID:              link.DocID.String(),
		IssueID:            optionalID(link.IssueID),
		ProjectID:          optionalID(link.ProjectID),
		CreatedByAccountID: optionalID(link.CreatedByAccountID),
		CreatedAt:          link.CreatedAt,
	}
}

func optionalID(id uuid.UUID) null.String {
	return null.NewString(id.String(), id != uuid.Nil)
}

func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		for _, index := range uniqueLinkIndexes {
			if pgErr.ConstraintName == index {
				return entity.ErrDocLinkExists
			}
		}
	}

	return fmt.Errorf("insert doc link: %w", err)
}

func (r *linkRepository) Create(ctx context.Context, link entity.StoredDocLink) error {
	if err := linkToModel(link).Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return translateWriteError(err)
	}

	return nil
}

func (r *linkRepository) Delete(ctx context.Context, workspaceID, docID, linkID uuid.UUID) error {
	removed, err := dbpostgres.WorkspaceDocLinks(
		dbpostgres.WorkspaceDocLinkWhere.ID.EQ(linkID.String()),
		dbpostgres.WorkspaceDocLinkWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceDocLinkWhere.DocID.EQ(docID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete doc link: %w", err)
	}

	if removed == 0 {
		return entity.ErrDocLinkNotFound
	}

	return nil
}

func (r *linkRepository) ListForDoc(
	ctx context.Context,
	workspaceID, docID uuid.UUID,
	scope entity.TeamScope,
) ([]entity.DocLink, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		linksForDocQuery,
		workspaceID.String(),
		docID.String(),
		scope.AllTeams,
		teamIDs(scope),
	)
	if err != nil {
		return nil, fmt.Errorf("list doc links: %w", err)
	}
	defer func() { _ = rows.Close() }()

	links := make([]entity.DocLink, 0)

	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, fmt.Errorf("scan doc link: %w", err)
		}

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read doc links: %w", err)
	}

	return links, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanLink(row scanner) (entity.DocLink, error) {
	var (
		link                                                  entity.DocLink
		id, docID, issueID, status, teamID, stateID, category string
		projectID                                             string
		archivedAt                                            sql.NullTime
	)

	if err := row.Scan(
		&id,
		&docID,
		&link.CreatedAt,
		&issueID,
		&link.Issue.ReferenceKey,
		&link.Issue.Number,
		&link.Issue.Title,
		&status,
		&teamID,
		&link.Issue.TeamKey,
		&stateID,
		&link.Issue.State.Name,
		&category,
		&link.Issue.State.Position,
		&projectID,
		&link.Project.Slug,
		&link.Project.Name,
		&archivedAt,
	); err != nil {
		return entity.DocLink{}, err
	}

	var err error

	if link.ID, err = uuid.Parse(id); err != nil {
		return entity.DocLink{}, fmt.Errorf("parse doc link id: %w", err)
	}

	if link.DocID, err = uuid.Parse(docID); err != nil {
		return entity.DocLink{}, fmt.Errorf("parse doc link doc id: %w", err)
	}

	link.CreatedAt = link.CreatedAt.UTC()

	if issueID == "" {
		return projectLink(link, projectID, archivedAt)
	}

	link.Target = entity.DocLinkTargetIssue
	link.Issue.Status = entity.IssueStatus(status)
	link.Issue.State.Category = entity.StateCategory(category)

	if link.Issue.ID, err = uuid.Parse(issueID); err != nil {
		return entity.DocLink{}, fmt.Errorf("parse linked issue id: %w", err)
	}

	if link.Issue.TeamID, err = uuid.Parse(teamID); err != nil {
		return entity.DocLink{}, fmt.Errorf("parse linked issue team id: %w", err)
	}

	if link.Issue.State.ID, err = uuid.Parse(stateID); err != nil {
		return entity.DocLink{}, fmt.Errorf("parse linked issue state id: %w", err)
	}

	return link, nil
}

func projectLink(link entity.DocLink, projectID string, archivedAt sql.NullTime) (entity.DocLink, error) {
	var err error

	link.Target = entity.DocLinkTargetProject

	if link.Project.ID, err = uuid.Parse(projectID); err != nil {
		return entity.DocLink{}, fmt.Errorf("parse linked project id: %w", err)
	}

	if archivedAt.Valid {
		archived := archivedAt.Time.UTC()
		link.Project.ArchivedAt = &archived
	}

	return link, nil
}

func teamIDs(scope entity.TeamScope) []string {
	ids := make([]string, 0, len(scope.TeamIDs))

	for _, id := range scope.TeamIDs {
		ids = append(ids, id.String())
	}

	return ids
}

package docrevision

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const summaryColumns = `
       r.id,
       r.workspace_id,
       r.doc_id,
       r.version,
       r.action,
       r.title,
       coalesce(r.parent_id::text, ''),
       r.visibility,
       coalesce(r.project_id::text, ''),
       coalesce(r.account_id::text, ''),
       coalesce(a.display_name, ''),
       coalesce(r.agent_id::text, ''),
       coalesce(r.connection_id::text, ''),
       r.source,
       r.created_at`

const listRevisionsQuery = `
SELECT` + summaryColumns + `
FROM workspace_doc_revisions r
LEFT JOIN accounts a ON a.id = r.account_id
WHERE r.doc_id = $1 AND r.workspace_id = $2
  AND ($3::integer IS NULL OR r.version < $3::integer)
ORDER BY r.version DESC
LIMIT $4`

const revisionByIDQuery = `
SELECT` + summaryColumns + `,
       r.content::text,
       r.markdown
FROM workspace_doc_revisions r
LEFT JOIN accounts a ON a.id = r.account_id
WHERE r.id = $1 AND r.doc_id = $2 AND r.workspace_id = $3`

func revisionToModel(revision entity.DocRevision) (*dbpostgres.WorkspaceDocRevision, error) {
	content, err := revision.Content.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode doc revision content: %w", err)
	}

	projectID := null.String{}
	if revision.Audience.Visibility == entity.KnowledgeVisibilityProject {
		projectID = null.StringFrom(revision.Audience.ProjectID.String())
	}

	return &dbpostgres.WorkspaceDocRevision{
		ID:           revision.ID.String(),
		WorkspaceID:  revision.WorkspaceID.String(),
		DocID:        revision.DocID.String(),
		Version:      revision.Version,
		Action:       string(revision.Action),
		Title:        revision.Title,
		Content:      types.JSON(content),
		Markdown:     revision.Markdown,
		ParentID:     optionalUUID(revision.ParentID),
		Visibility:   string(revision.Audience.Visibility),
		ProjectID:    projectID,
		AccountID:    null.NewString(revision.By.AccountID.String(), revision.By.AccountID != uuid.Nil),
		AgentID:      optionalUUID(revision.By.AgentID),
		ConnectionID: optionalUUID(revision.By.ConnectionID),
		Source:       string(revision.By.Source),
		CreatedAt:    revision.CreatedAt,
	}, nil
}

func optionalUUID(id *uuid.UUID) null.String {
	if id == nil {
		return null.String{}
	}

	return null.StringFrom(id.String())
}

type summaryRow struct {
	id, workspace, doc, action, parent, visibility, project, account, agent, connection, source string
}

func (row *summaryRow) targets(summary *entity.DocRevisionSummary) []any {
	return []any{
		&row.id, &row.workspace, &row.doc, &summary.Version, &row.action, &summary.Title, &row.parent,
		&row.visibility, &row.project, &row.account, &summary.AccountName, &row.agent, &row.connection,
		&row.source, &summary.CreatedAt,
	}
}

func (row *summaryRow) fill(summary *entity.DocRevisionSummary) error {
	var err error

	if summary.ID, err = uuid.Parse(row.id); err != nil {
		return fmt.Errorf("parse doc revision id: %w", err)
	}

	if summary.WorkspaceID, err = uuid.Parse(row.workspace); err != nil {
		return fmt.Errorf("parse doc revision workspace id: %w", err)
	}

	if summary.DocID, err = uuid.Parse(row.doc); err != nil {
		return fmt.Errorf("parse doc revision doc id: %w", err)
	}

	if summary.ParentID, err = parseOptional(row.parent); err != nil {
		return fmt.Errorf("parse doc revision parent id: %w", err)
	}

	if summary.By.AccountID, err = parseOrNil(row.account); err != nil {
		return fmt.Errorf("parse doc revision account id: %w", err)
	}

	if summary.By.AgentID, err = parseOptional(row.agent); err != nil {
		return fmt.Errorf("parse doc revision agent id: %w", err)
	}

	if summary.By.ConnectionID, err = parseOptional(row.connection); err != nil {
		return fmt.Errorf("parse doc revision connection id: %w", err)
	}

	projectID, err := parseOrNil(row.project)
	if err != nil {
		return fmt.Errorf("parse doc revision project id: %w", err)
	}

	summary.Action = entity.DocRevisionAction(row.action)
	summary.Audience = entity.KnowledgeAudience{Visibility: entity.KnowledgeVisibility(row.visibility), ProjectID: projectID}
	summary.By.Source = entity.RevisionSource(row.source)

	return nil
}

func parseOptional(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

func parseOrNil(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(raw)
}

type revisionRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.DocRevision {
	return &revisionRepository{db: db}
}

func (r *revisionRepository) Record(ctx context.Context, revision entity.DocRevision) error {
	model, err := revisionToModel(revision)
	if err != nil {
		return err
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return fmt.Errorf("insert doc revision: %w", err)
	}

	return nil
}

func (r *revisionRepository) List(
	ctx context.Context,
	workspaceID, docID uuid.UUID,
	page entity.DocRevisionPage,
) ([]entity.DocRevisionSummary, error) {
	var before sql.NullInt64
	if page.Cursor != nil {
		before = sql.NullInt64{Int64: int64(page.Cursor.Version), Valid: true}
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listRevisionsQuery, docID.String(), workspaceID.String(), before, page.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list doc revisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []entity.DocRevisionSummary

	for rows.Next() {
		var (
			summary entity.DocRevisionSummary
			raw     summaryRow
		)

		if err := rows.Scan(raw.targets(&summary)...); err != nil {
			return nil, fmt.Errorf("scan doc revision: %w", err)
		}

		if err := raw.fill(&summary); err != nil {
			return nil, err
		}

		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read doc revisions: %w", err)
	}

	return summaries, nil
}

func (r *revisionRepository) GetByID(
	ctx context.Context,
	workspaceID, docID, revisionID uuid.UUID,
) (entity.DocRevision, error) {
	var (
		revision entity.DocRevision
		raw      summaryRow
		content  string
	)

	row := r.db.Querier(ctx).QueryRowContext(
		ctx, revisionByIDQuery, revisionID.String(), docID.String(), workspaceID.String(),
	)

	if err := row.Scan(append(raw.targets(&revision.DocRevisionSummary), &content, &revision.Markdown)...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.DocRevision{}, entity.ErrDocRevisionNotFound
		}

		return entity.DocRevision{}, fmt.Errorf("find doc revision: %w", err)
	}

	if err := raw.fill(&revision.DocRevisionSummary); err != nil {
		return entity.DocRevision{}, err
	}

	decoded, err := entity.DecodeDocument([]byte(content))
	if err != nil {
		return entity.DocRevision{}, fmt.Errorf("decode doc revision content: %w", err)
	}

	revision.Content = decoded

	return revision, nil
}

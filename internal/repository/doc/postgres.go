package doc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
       d.id,
       d.workspace_id,
       coalesce(d.parent_id::text, ''),
       d.title,
       d.visibility,
       coalesce(d.project_id::text, ''),
       coalesce(d.author_account_id::text, ''),
       coalesce(a.display_name, ''),
       d.status,
       d.archived_at,
       d.version,
       d.created_at,
       d.updated_at`

const docColumns = summaryColumns + `,
       d.content::text,
       d.markdown,
       coalesce((SELECT string_agg(DISTINCT r.account_id::text, ',')
                 FROM workspace_doc_revisions r
                 WHERE r.doc_id = d.id AND r.account_id IS NOT NULL), '')`

const docByIDQuery = `
SELECT` + docColumns + `
FROM workspace_docs d
LEFT JOIN accounts a ON a.id = d.author_account_id
WHERE d.id = $1 AND d.workspace_id = $2`

const lockDocQuery = docByIDQuery + `
FOR UPDATE OF d`

const visibleToReader = `
  AND d.status = 'active'
  AND (d.visibility = 'workspace'
       OR (d.visibility = 'project' AND ($2::boolean IS TRUE OR d.project_id = ANY($3::uuid[])))
       OR (d.visibility = 'personal' AND d.author_account_id = $4::uuid))`

const listDocsQuery = `
SELECT` + summaryColumns + `
FROM workspace_docs d
LEFT JOIN accounts a ON a.id = d.author_account_id
WHERE d.workspace_id = $1` + visibleToReader + `
  AND ($5::uuid IS NULL OR d.parent_id = $5::uuid)
  AND ($6::boolean IS FALSE OR d.parent_id IS NULL)
  AND ($7::timestamptz IS NULL OR (d.updated_at, d.id) < ($7::timestamptz, $8::uuid))
  AND ($10::uuid IS NULL OR d.project_id = $10::uuid)
  AND ($11::uuid IS NULL OR d.id IN (
      WITH RECURSIVE below (id, depth) AS (
          SELECT id, 0 FROM workspace_docs WHERE id = $11::uuid AND workspace_id = $1
          UNION ALL
          SELECT child.id, below.depth + 1
          FROM workspace_docs child
          JOIN below ON child.parent_id = below.id
          WHERE child.workspace_id = $1 AND below.depth < $12
      )
      SELECT id FROM below WHERE depth > 0))
  AND ($13::uuid IS NULL OR EXISTS (
      SELECT 1 FROM workspace_doc_links l WHERE l.doc_id = d.id AND l.issue_id = $13::uuid))
  AND ($14::uuid IS NULL OR EXISTS (
      SELECT 1 FROM workspace_doc_links l WHERE l.doc_id = d.id AND l.project_id = $14::uuid))
ORDER BY d.updated_at DESC, d.id DESC
LIMIT $9`

const docQueryTerms = `(websearch_to_tsquery('english', $5) ||
       coalesce(to_tsquery('simple', nullif($6, '') || ':*'), ''::tsquery))`

const searchDocsQuery = `
SELECT` + summaryColumns + `,
       ts_headline('english', d.markdown, ` + docQueryTerms + `, $9)
FROM workspace_docs d
LEFT JOIN accounts a ON a.id = d.author_account_id
WHERE d.workspace_id = $1` + visibleToReader + `
  AND d.search_document @@ ` + docQueryTerms + `
  AND ($7::uuid IS NULL OR d.project_id = $7::uuid)
ORDER BY ts_rank_cd(d.search_document, ` + docQueryTerms + `) DESC, d.updated_at DESC, d.id DESC
LIMIT $8`

const excerptOptions = "StartSel=**, StopSel=**, MaxFragments=2, MaxWords=24, MinWords=8, FragmentDelimiter= … "

const ancestryQuery = `
WITH RECURSIVE ancestry (id, parent_id, depth) AS (
    SELECT id, parent_id, 1 FROM workspace_docs WHERE id = $1 AND workspace_id = $2
    UNION ALL
    SELECT d.id, d.parent_id, ancestry.depth + 1
    FROM workspace_docs d
    JOIN ancestry ON d.id = ancestry.parent_id
    WHERE d.workspace_id = $2 AND ancestry.depth <= $3
)
SELECT id::text FROM ancestry ORDER BY depth`

const subtreeHeightQuery = `
WITH RECURSIVE subtree (id, depth) AS (
    SELECT id, 0 FROM workspace_docs WHERE id = $1 AND workspace_id = $2
    UNION ALL
    SELECT d.id, subtree.depth + 1
    FROM workspace_docs d
    JOIN subtree ON d.parent_id = subtree.id
    WHERE d.workspace_id = $2 AND subtree.depth <= $3
)
SELECT coalesce(max(depth), 0) FROM subtree`

const activeChildrenQuery = `
SELECT EXISTS (
    SELECT 1 FROM workspace_docs
    WHERE parent_id = $1 AND workspace_id = $2 AND status = 'active'
)`

func docToModel(doc entity.Doc) (*dbpostgres.WorkspaceDoc, error) {
	content, err := doc.Content.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode doc content: %w", err)
	}

	return &dbpostgres.WorkspaceDoc{
		ID:              doc.ID.String(),
		WorkspaceID:     doc.WorkspaceID.String(),
		ParentID:        optionalUUID(doc.ParentID),
		Title:           doc.Title,
		Content:         types.JSON(content),
		Markdown:        doc.Markdown,
		Visibility:      string(doc.Audience.Visibility),
		ProjectID:       audienceProject(doc.Audience),
		AuthorAccountID: null.NewString(doc.AuthorID.String(), doc.AuthorID != uuid.Nil),
		Status:          string(doc.Status),
		ArchivedAt:      null.TimeFromPtr(doc.ArchivedAt),
		Version:         doc.Version,
		CreatedAt:       doc.CreatedAt,
		UpdatedAt:       doc.UpdatedAt,
	}, nil
}

func optionalUUID(id *uuid.UUID) null.String {
	if id == nil {
		return null.String{}
	}

	return null.StringFrom(id.String())
}

func audienceProject(audience entity.KnowledgeAudience) null.String {
	if audience.Visibility != entity.KnowledgeVisibilityProject {
		return null.String{}
	}

	return null.StringFrom(audience.ProjectID.String())
}

type scanner interface {
	Scan(dest ...any) error
}

type summaryRow struct {
	id, workspace, parent, visibility, project, author, status string
	archivedAt                                                 sql.NullTime
}

func (row *summaryRow) targets(summary *entity.DocSummary) []any {
	return []any{
		&row.id, &row.workspace, &row.parent, &summary.Title, &row.visibility, &row.project,
		&row.author, &summary.AuthorName, &row.status, &row.archivedAt, &summary.Version,
		&summary.CreatedAt, &summary.UpdatedAt,
	}
}

func (row *summaryRow) fill(summary *entity.DocSummary) error {
	var err error

	if summary.ID, err = uuid.Parse(row.id); err != nil {
		return fmt.Errorf("parse doc id: %w", err)
	}

	if summary.WorkspaceID, err = uuid.Parse(row.workspace); err != nil {
		return fmt.Errorf("parse doc workspace id: %w", err)
	}

	if summary.ParentID, err = parseOptional(row.parent); err != nil {
		return fmt.Errorf("parse doc parent id: %w", err)
	}

	if summary.AuthorID, err = parseOrNil(row.author); err != nil {
		return fmt.Errorf("parse doc author id: %w", err)
	}

	projectID, err := parseOrNil(row.project)
	if err != nil {
		return fmt.Errorf("parse doc project id: %w", err)
	}

	summary.Audience = entity.KnowledgeAudience{Visibility: entity.KnowledgeVisibility(row.visibility), ProjectID: projectID}
	summary.Status = entity.DocStatus(row.status)
	summary.CreatedAt = summary.CreatedAt.UTC()
	summary.UpdatedAt = summary.UpdatedAt.UTC()

	if row.archivedAt.Valid {
		archivedAt := row.archivedAt.Time.UTC()
		summary.ArchivedAt = &archivedAt
	}

	return nil
}

func scanSummary(row scanner) (entity.DocSummary, error) {
	var (
		summary entity.DocSummary
		raw     summaryRow
	)

	if err := row.Scan(raw.targets(&summary)...); err != nil {
		return entity.DocSummary{}, err
	}

	if err := raw.fill(&summary); err != nil {
		return entity.DocSummary{}, err
	}

	return summary, nil
}

func scanDoc(row scanner) (entity.Doc, error) {
	var (
		doc                   entity.Doc
		raw                   summaryRow
		content, contributors string
	)

	if err := row.Scan(append(raw.targets(&doc.DocSummary), &content, &doc.Markdown, &contributors)...); err != nil {
		return entity.Doc{}, err
	}

	if err := raw.fill(&doc.DocSummary); err != nil {
		return entity.Doc{}, err
	}

	decoded, err := entity.DecodeDocument([]byte(content))
	if err != nil {
		return entity.Doc{}, fmt.Errorf("decode doc content: %w", err)
	}

	doc.Content = decoded

	if contributors != "" {
		for _, raw := range strings.Split(contributors, ",") {
			contributor, err := uuid.Parse(raw)
			if err != nil {
				return entity.Doc{}, fmt.Errorf("parse doc contributor id: %w", err)
			}

			doc.ContributorIDs = append(doc.ContributorIDs, contributor)
		}
	}

	return doc, nil
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

type docRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Doc {
	return &docRepository{db: db}
}

func (r *docRepository) Create(ctx context.Context, doc entity.Doc) error {
	model, err := docToModel(doc)
	if err != nil {
		return err
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return fmt.Errorf("insert doc: %w", err)
	}

	return nil
}

func (r *docRepository) GetByID(ctx context.Context, workspaceID, docID uuid.UUID) (entity.Doc, error) {
	return r.find(ctx, docByIDQuery, workspaceID, docID)
}

func (r *docRepository) LockByID(ctx context.Context, workspaceID, docID uuid.UUID) (entity.Doc, error) {
	return r.find(ctx, lockDocQuery, workspaceID, docID)
}

func (r *docRepository) find(ctx context.Context, query string, workspaceID, docID uuid.UUID) (entity.Doc, error) {
	doc, err := scanDoc(r.db.Querier(ctx).QueryRowContext(ctx, query, docID.String(), workspaceID.String()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Doc{}, entity.ErrDocNotFound
		}

		return entity.Doc{}, fmt.Errorf("find doc: %w", err)
	}

	return doc, nil
}

func (r *docRepository) Update(ctx context.Context, doc entity.Doc, expectedVersion int) error {
	model, err := docToModel(doc)
	if err != nil {
		return err
	}

	updated, err := dbpostgres.WorkspaceDocs(
		dbpostgres.WorkspaceDocWhere.ID.EQ(model.ID),
		dbpostgres.WorkspaceDocWhere.WorkspaceID.EQ(model.WorkspaceID),
		dbpostgres.WorkspaceDocWhere.Version.EQ(expectedVersion),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceDocColumns.ParentID:   model.ParentID,
		dbpostgres.WorkspaceDocColumns.Title:      model.Title,
		dbpostgres.WorkspaceDocColumns.Content:    model.Content,
		dbpostgres.WorkspaceDocColumns.Markdown:   model.Markdown,
		dbpostgres.WorkspaceDocColumns.Visibility: model.Visibility,
		dbpostgres.WorkspaceDocColumns.ProjectID:  model.ProjectID,
		dbpostgres.WorkspaceDocColumns.Status:     model.Status,
		dbpostgres.WorkspaceDocColumns.ArchivedAt: model.ArchivedAt,
		dbpostgres.WorkspaceDocColumns.Version:    model.Version,
		dbpostgres.WorkspaceDocColumns.UpdatedAt:  model.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("update doc: %w", err)
	}

	if updated == 0 {
		return entity.ErrDocStale
	}

	return nil
}

func (r *docRepository) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	filter repository.DocFilter,
	page entity.DocPage,
) ([]entity.DocSummary, error) {
	var (
		after   sql.NullTime
		afterID sql.NullString
	)

	if page.Cursor != nil {
		after = sql.NullTime{Time: page.Cursor.UpdatedAt, Valid: true}
		afterID = sql.NullString{String: page.Cursor.DocID.String(), Valid: true}
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		listDocsQuery,
		workspaceID.String(),
		filter.AllProjects,
		idStrings(filter.ProjectIDs),
		nullableAuthor(filter.PersonalAuthorID),
		nullableID(filter.ParentID),
		filter.Root,
		after,
		afterID,
		page.Limit,
		nullableID(filter.ProjectID),
		nullableID(filter.WithinID),
		entity.DocTreeMaxDepth,
		nullableID(filter.LinkedIssueID),
		nullableID(filter.LinkedProjectID),
	)
	if err != nil {
		return nil, fmt.Errorf("list docs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []entity.DocSummary

	for rows.Next() {
		summary, err := scanSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("scan doc: %w", err)
		}

		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read docs: %w", err)
	}

	return summaries, nil
}

func (r *docRepository) Search(
	ctx context.Context,
	workspaceID uuid.UUID,
	search repository.DocSearch,
) ([]entity.DocMatch, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		searchDocsQuery,
		workspaceID.String(),
		search.AllProjects,
		idStrings(search.ProjectIDs),
		nullableAuthor(search.PersonalAuthorID),
		search.Query.Stemmed,
		search.Query.Prefix,
		nullableID(search.ProjectID),
		search.Limit,
		excerptOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("search docs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var matches []entity.DocMatch

	for rows.Next() {
		var (
			match entity.DocMatch
			raw   summaryRow
		)

		if err := rows.Scan(append(raw.targets(&match.DocSummary), &match.Excerpt)...); err != nil {
			return nil, fmt.Errorf("scan doc match: %w", err)
		}

		if err := raw.fill(&match.DocSummary); err != nil {
			return nil, err
		}

		match.Excerpt = strings.TrimSpace(match.Excerpt)
		matches = append(matches, match)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read doc matches: %w", err)
	}

	return matches, nil
}

func idStrings(ids []uuid.UUID) []string {
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		strs = append(strs, id.String())
	}

	return strs
}

func nullableID(id *uuid.UUID) sql.NullString {
	if id == nil {
		return sql.NullString{}
	}

	return sql.NullString{String: id.String(), Valid: true}
}

func nullableAuthor(authorID uuid.UUID) sql.NullString {
	return sql.NullString{String: authorID.String(), Valid: authorID != uuid.Nil}
}

func (r *docRepository) Ancestry(ctx context.Context, workspaceID, docID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, ancestryQuery, docID.String(), workspaceID.String(), entity.DocTreeMaxDepth,
	)
	if err != nil {
		return nil, fmt.Errorf("walk doc ancestry: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ancestry []uuid.UUID

	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan doc ancestor: %w", err)
		}

		ancestor, err := uuid.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse doc ancestor id: %w", err)
		}

		ancestry = append(ancestry, ancestor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read doc ancestry: %w", err)
	}

	if len(ancestry) == 0 {
		return nil, entity.ErrDocNotFound
	}

	return ancestry, nil
}

func (r *docRepository) SubtreeHeight(ctx context.Context, workspaceID, docID uuid.UUID) (int, error) {
	var height int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, subtreeHeightQuery, docID.String(), workspaceID.String(), entity.DocTreeMaxDepth,
	).Scan(&height); err != nil {
		return 0, fmt.Errorf("measure doc subtree: %w", err)
	}

	return height, nil
}

func (r *docRepository) HasActiveChildren(ctx context.Context, workspaceID, docID uuid.UUID) (bool, error) {
	var exists bool

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, activeChildrenQuery, docID.String(), workspaceID.String(),
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("find active child docs: %w", err)
	}

	return exists, nil
}

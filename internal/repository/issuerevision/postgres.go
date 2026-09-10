package issuerevision

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const revisionColumns = `
       r.id,
       r.workspace_id,
       r.issue_id,
       r.issue_version,
       r.doc::text,
       r.markdown,
       coalesce(r.author_account_id::text, ''),
       coalesce(a.display_name, ''),
       r.source,
       r.created_at`

// recordRevisionQuery keeps one row per version of an issue. A retried write that lands twice
// therefore leaves one entry rather than a doubled history.
const recordRevisionQuery = `
INSERT INTO workspace_issue_description_revisions
    (id, workspace_id, issue_id, issue_version, doc, markdown, author_account_id, source, created_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6, nullif($7, '')::uuid, $8, $9)
ON CONFLICT (issue_id, issue_version) DO NOTHING`

const listRevisionsQuery = `
SELECT` + revisionColumns + `
FROM workspace_issue_description_revisions r
LEFT JOIN accounts a ON a.id = r.author_account_id
WHERE r.issue_id = $1 AND r.workspace_id = $2
ORDER BY r.created_at DESC, r.id DESC
LIMIT $3`

const revisionByIDQuery = `
SELECT` + revisionColumns + `
FROM workspace_issue_description_revisions r
LEFT JOIN accounts a ON a.id = r.author_account_id
WHERE r.id = $1 AND r.workspace_id = $2`

type revisionRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueRevision {
	return &revisionRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRevision(row scanner) (entity.IssueDescriptionRevision, error) {
	var (
		revision                     entity.IssueDescriptionRevision
		id, workspace, issue, author string
		document, source             string
	)

	if err := row.Scan(
		&id, &workspace, &issue, &revision.IssueVersion, &document, &revision.Markdown,
		&author, &revision.AuthorName, &source, &revision.CreatedAt,
	); err != nil {
		return entity.IssueDescriptionRevision{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.IssueDescriptionRevision{}, fmt.Errorf("parse revision id: %w", err)
	}

	revision.ID = parsed

	if revision.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.IssueDescriptionRevision{}, fmt.Errorf("parse revision workspace id: %w", err)
	}

	if revision.IssueID, err = uuid.Parse(issue); err != nil {
		return entity.IssueDescriptionRevision{}, fmt.Errorf("parse revision issue id: %w", err)
	}

	if author != "" {
		if revision.AuthorAccountID, err = uuid.Parse(author); err != nil {
			return entity.IssueDescriptionRevision{}, fmt.Errorf("parse revision author id: %w", err)
		}
	}

	if revision.Doc, err = entity.DecodeDocument([]byte(document)); err != nil {
		return entity.IssueDescriptionRevision{}, fmt.Errorf("decode revision document: %w", err)
	}

	revision.Source = entity.RevisionSource(source)

	return revision, nil
}

func (r *revisionRepository) Record(
	ctx context.Context,
	revision entity.IssueDescriptionRevision,
) error {
	id := revision.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	encoded, err := revision.Doc.Encode()
	if err != nil {
		return fmt.Errorf("encode revision document: %w", err)
	}

	author := ""
	if revision.AuthorAccountID != uuid.Nil {
		author = revision.AuthorAccountID.String()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, recordRevisionQuery,
		id.String(), revision.WorkspaceID.String(), revision.IssueID.String(),
		revision.IssueVersion, string(encoded), revision.Markdown, author,
		string(revision.Source), revision.CreatedAt,
	); err != nil {
		return fmt.Errorf("record description revision: %w", err)
	}

	return nil
}

func (r *revisionRepository) List(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	limit int,
) ([]entity.IssueDescriptionRevision, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listRevisionsQuery, issueID.String(), workspaceID.String(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list description revisions: %w", err)
	}

	defer func() { _ = rows.Close() }()

	revisions := make([]entity.IssueDescriptionRevision, 0, limit)

	for rows.Next() {
		revision, err := scanRevision(rows)
		if err != nil {
			return nil, fmt.Errorf("read description revision: %w", err)
		}

		revisions = append(revisions, revision)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read description revisions: %w", err)
	}

	return revisions, nil
}

func (r *revisionRepository) GetByID(
	ctx context.Context,
	workspaceID, revisionID uuid.UUID,
) (entity.IssueDescriptionRevision, error) {
	revision, err := scanRevision(r.db.Querier(ctx).QueryRowContext(
		ctx, revisionByIDQuery, revisionID.String(), workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IssueDescriptionRevision{}, entity.ErrIssueRevisionNotFound
		}

		return entity.IssueDescriptionRevision{}, fmt.Errorf("find description revision: %w", err)
	}

	return revision, nil
}

package scm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type linkRepository struct {
	db *postgres.Client
}

func NewCodeLink(db *postgres.Client) repository.CodeLink {
	return &linkRepository{db: db}
}

const linkColumns = `
    id, workspace_id, issue_id,
    coalesce(repository_id, '00000000-0000-0000-0000-000000000000'::uuid),
    provider, repository_name, kind, external_id, number, title, url, state, checks, author,
    head_branch, base_branch, paths, detected_in, resolving, merge_commit_sha,
    source_updated_at,
    merged_at, closed_at, created_at, updated_at`

func scanLink(row interface{ Scan(...any) error }) (entity.CodeLink, error) {
	var (
		link  entity.CodeLink
		paths types.StringArray
	)

	err := row.Scan(
		&link.ID,
		&link.WorkspaceID,
		&link.IssueID,
		&link.RepositoryID,
		&link.Provider,
		&link.RepositoryName,
		&link.Kind,
		&link.ExternalID,
		&link.Number,
		&link.Title,
		&link.URL,
		&link.State,
		&link.Checks,
		&link.Author,
		&link.HeadBranch,
		&link.BaseBranch,
		&paths,
		&link.DetectedIn,
		&link.Resolving,
		&link.MergeCommitSHA,
		&link.SourceUpdatedAt,
		&link.MergedAt,
		&link.ClosedAt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		return entity.CodeLink{}, err
	}

	link.Paths = paths

	return link, nil
}

const upsertLinkQuery = `
INSERT INTO workspace_code_links (
    id, workspace_id, issue_id, repository_id, provider, repository_name, kind, external_id,
    number, title, url, state, checks, author, head_branch, base_branch, paths, detected_in,
    resolving, merge_commit_sha, source_updated_at, merged_at, closed_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
          $19, $20, $21, $22, $23)
ON CONFLICT (issue_id, provider, repository_name, kind, external_id) DO UPDATE
SET repository_id = excluded.repository_id,
    number = excluded.number,
    title = excluded.title,
    url = excluded.url,
    state = excluded.state,
    checks = excluded.checks,
    author = excluded.author,
    head_branch = excluded.head_branch,
    base_branch = excluded.base_branch,
    paths = excluded.paths,
    resolving = excluded.resolving,
    merge_commit_sha = excluded.merge_commit_sha,
    source_updated_at = excluded.source_updated_at,
    merged_at = excluded.merged_at,
    closed_at = excluded.closed_at,
    updated_at = now()
WHERE workspace_code_links.source_updated_at IS NULL
   OR excluded.source_updated_at IS NULL
   OR excluded.source_updated_at >= workspace_code_links.source_updated_at
RETURNING` + linkColumns

const readLinkQuery = `
SELECT` + linkColumns + `
FROM workspace_code_links
WHERE issue_id = $1 AND provider = $2 AND repository_name = $3 AND kind = $4
  AND external_id = $5`

func (r *linkRepository) Upsert(
	ctx context.Context,
	link entity.CodeLink,
) (entity.CodeLink, error) {
	if link.ID == uuid.Nil {
		link.ID = uuid.New()
	}

	stored, err := scanLink(r.db.Querier(ctx).QueryRowContext(
		ctx,
		upsertLinkQuery,
		link.ID,
		link.WorkspaceID,
		link.IssueID,
		repositoryOrNil(link.RepositoryID),
		link.Provider,
		link.RepositoryName,
		link.Kind,
		link.ExternalID,
		link.Number,
		link.Title,
		link.URL,
		link.State,
		link.Checks,
		link.Author,
		link.HeadBranch,
		link.BaseBranch,
		types.StringArray(link.Paths),
		link.DetectedIn,
		link.Resolving,
		link.MergeCommitSHA,
		link.SourceUpdatedAt,
		link.MergedAt,
		link.ClosedAt,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return scanLinkOrFail(r.db.Querier(ctx).QueryRowContext(
			ctx,
			readLinkQuery,
			link.IssueID,
			link.Provider,
			link.RepositoryName,
			link.Kind,
			link.ExternalID,
		))
	}

	if err != nil {
		return entity.CodeLink{}, fmt.Errorf("record linked change: %w", err)
	}

	return stored, nil
}

func scanLinkOrFail(row interface{ Scan(...any) error }) (entity.CodeLink, error) {
	link, err := scanLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.CodeLink{}, entity.ErrCodeLinkNotFound
	}

	if err != nil {
		return entity.CodeLink{}, fmt.Errorf("read linked change: %w", err)
	}

	return link, nil
}

func repositoryOrNil(repositoryID uuid.UUID) any {
	if repositoryID == uuid.Nil {
		return nil
	}

	return repositoryID
}

const listLinksByIssueQuery = `
SELECT` + linkColumns + `
FROM workspace_code_links
WHERE workspace_id = $1 AND issue_id = $2
ORDER BY created_at DESC, id`

func (r *linkRepository) ListByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.CodeLink, error) {
	return r.collect(ctx, listLinksByIssueQuery, workspaceID, issueID)
}

const listLinksByExternalQuery = `
SELECT` + linkColumns + `
FROM workspace_code_links
WHERE workspace_id = $1 AND provider = $2 AND repository_name = $3 AND external_id = $4
ORDER BY created_at, id`

func (r *linkRepository) ListByExternalID(
	ctx context.Context,
	workspaceID uuid.UUID,
	provider entity.SCMProvider,
	repositoryName, externalID string,
) ([]entity.CodeLink, error) {
	return r.collect(ctx, listLinksByExternalQuery, workspaceID, provider, repositoryName, externalID)
}

func (r *linkRepository) collect(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.CodeLink, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list linked changes: %w", err)
	}

	defer func() { _ = rows.Close() }()

	links := make([]entity.CodeLink, 0)

	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, fmt.Errorf("scan linked change: %w", err)
		}

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read linked changes: %w", err)
	}

	return links, nil
}

const claimTransitionQuery = `
INSERT INTO workspace_code_link_transitions (link_id, transition, issue_id, applied_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (link_id, transition) DO NOTHING`

func (r *linkRepository) ClaimTransition(
	ctx context.Context,
	linkID uuid.UUID,
	transition entity.CodeChangeState,
	issueID uuid.UUID,
	at time.Time,
) (bool, error) {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, claimTransitionQuery, linkID, transition, issueID, at,
	)
	if err != nil {
		return false, fmt.Errorf("claim a linked change transition: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read affected rows: %w", err)
	}

	return affected > 0, nil
}

const deleteLinkQuery = `
DELETE FROM workspace_code_links
WHERE workspace_id = $1 AND issue_id = $2 AND id = $3
RETURNING` + linkColumns

func (r *linkRepository) Delete(
	ctx context.Context,
	workspaceID, issueID, linkID uuid.UUID,
) (entity.CodeLink, error) {
	return scanLinkOrFail(r.db.Querier(ctx).QueryRowContext(
		ctx,
		deleteLinkQuery,
		workspaceID,
		issueID,
		linkID,
	))
}

const detachLinksQuery = `
UPDATE workspace_code_links
SET repository_id = NULL, updated_at = now()
WHERE repository_id = $1`

func (r *linkRepository) DetachRepository(ctx context.Context, repositoryID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, detachLinksQuery, repositoryID); err != nil {
		return fmt.Errorf("detach linked changes from a repository: %w", err)
	}

	return nil
}

const setChecksQuery = `
UPDATE workspace_code_links
SET checks = $5, updated_at = now()
WHERE workspace_id = $1 AND provider = $2 AND repository_name = $3 AND external_id = $4`

func (r *linkRepository) SetChecks(
	ctx context.Context,
	workspaceID uuid.UUID,
	provider entity.SCMProvider,
	repositoryName, externalID string,
	checks entity.CodeChecks,
) (int, error) {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, setChecksQuery, workspaceID, provider, repositoryName, externalID, checks,
	)
	if err != nil {
		return 0, fmt.Errorf("record the checks on a linked change: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read affected rows: %w", err)
	}

	return int(affected), nil
}

const listLinksByRepositoryQuery = `
SELECT` + linkColumns + `
FROM workspace_code_links
WHERE repository_id = $1 AND merge_commit_sha <> ''
ORDER BY created_at, id`

func (r *linkRepository) ListByRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
) ([]entity.CodeLink, error) {
	return r.collect(ctx, listLinksByRepositoryQuery, repositoryID)
}

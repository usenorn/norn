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
    head_branch, head_sha, base_branch, paths, detected_in, resolving, merge_commit_sha,
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
		&link.HeadSHA,
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
    number, title, url, state, checks, author, head_branch, head_sha, base_branch, paths,
    detected_in, resolving, merge_commit_sha, source_updated_at, merged_at, closed_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
          $19, $20, $21, $22, $23, $24)
ON CONFLICT (issue_id, provider, repository_name, kind, external_id) DO UPDATE
SET repository_id = excluded.repository_id,
    number = excluded.number,
    title = excluded.title,
    url = excluded.url,
    state = excluded.state,
    checks = excluded.checks,
    author = excluded.author,
    head_branch = excluded.head_branch,
    head_sha = excluded.head_sha,
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
		link.HeadSHA,
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

const claimAnnouncementQuery = `
UPDATE workspace_code_links
SET announced_at = $2, updated_at = now()
WHERE id = $1 AND announced_at IS NULL`

func (r *linkRepository) ClaimAnnouncement(
	ctx context.Context,
	linkID uuid.UUID,
	at time.Time,
) (bool, error) {
	result, err := r.db.Querier(ctx).ExecContext(ctx, claimAnnouncementQuery, linkID, at)
	if err != nil {
		return false, fmt.Errorf("claim a linked change announcement: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read affected rows: %w", err)
	}

	return affected > 0, nil
}

const releaseAnnouncementQuery = `
UPDATE workspace_code_links
SET announced_at = NULL, updated_at = now()
WHERE id = $1`

func (r *linkRepository) ReleaseAnnouncement(ctx context.Context, linkID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, releaseAnnouncementQuery, linkID); err != nil {
		return fmt.Errorf("release a linked change announcement: %w", err)
	}

	return nil
}

const claimTransitionQuery = `
INSERT INTO workspace_code_link_transitions (link_id, transition, issue_id, state_id, applied_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (link_id, transition) DO NOTHING`

const deferTransitionQuery = `
UPDATE workspace_code_link_transitions
SET status = 'deferred', blocked_by = $3, deferred_at = $4
WHERE link_id = $1 AND transition = $2`

const settleTransitionQuery = `
UPDATE workspace_code_link_transitions
SET status = 'applied', blocked_by = '', deferred_at = NULL
WHERE link_id = $1 AND transition = $2`

const deferredTransitionsQuery = `
SELECT link_id, transition, issue_id, coalesce(state_id::text, ''), status, blocked_by, deferred_at
FROM workspace_code_link_transitions
WHERE issue_id = $1 AND status = 'deferred'
ORDER BY deferred_at, link_id`

func (r *linkRepository) ClaimTransition(
	ctx context.Context,
	linkID uuid.UUID,
	transition entity.CodeChangeState,
	issueID, stateID uuid.UUID,
	at time.Time,
) (bool, error) {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, claimTransitionQuery, linkID, transition, issueID, stateID, at,
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

func (r *linkRepository) DeferTransition(
	ctx context.Context,
	linkID uuid.UUID,
	transition entity.CodeChangeState,
	blockedBy entity.CodeTransitionBlock,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, deferTransitionQuery, linkID, transition, string(blockedBy), at,
	); err != nil {
		return fmt.Errorf("defer a linked change transition: %w", err)
	}

	return nil
}

func (r *linkRepository) SettleTransition(
	ctx context.Context,
	linkID uuid.UUID,
	transition entity.CodeChangeState,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, settleTransitionQuery, linkID, transition,
	); err != nil {
		return fmt.Errorf("settle a linked change transition: %w", err)
	}

	return nil
}

func (r *linkRepository) ListDeferredTransitions(
	ctx context.Context,
	issueID uuid.UUID,
) ([]entity.CodeTransition, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, deferredTransitionsQuery, issueID)
	if err != nil {
		return nil, fmt.Errorf("list deferred transitions: %w", err)
	}

	defer func() { _ = rows.Close() }()

	deferred := make([]entity.CodeTransition, 0)

	for rows.Next() {
		var (
			pending    entity.CodeTransition
			transition string
			stateID    string
			status     string
			blockedBy  string
			deferredAt sql.NullTime
		)

		if err := rows.Scan(
			&pending.LinkID,
			&transition,
			&pending.IssueID,
			&stateID,
			&status,
			&blockedBy,
			&deferredAt,
		); err != nil {
			return nil, fmt.Errorf("scan deferred transition: %w", err)
		}

		pending.Transition = entity.CodeChangeState(transition)
		pending.Status = entity.CodeTransitionStatus(status)
		pending.BlockedBy = entity.CodeTransitionBlock(blockedBy)

		if deferredAt.Valid {
			pending.DeferredAt = &deferredAt.Time
		}

		if stateID != "" {
			if pending.StateID, err = uuid.Parse(stateID); err != nil {
				return nil, fmt.Errorf("parse deferred transition state id: %w", err)
			}
		}

		deferred = append(deferred, pending)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deferred transitions: %w", err)
	}

	return deferred, nil
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

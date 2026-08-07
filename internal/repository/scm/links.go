package scm

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

type linkRepository struct {
	db *postgres.Client
}

func NewCodeLink(db *postgres.Client) repository.CodeLink {
	return &linkRepository{db: db}
}

const linkColumns = `
    id, workspace_id, issue_id,
    coalesce(connection_id, '00000000-0000-0000-0000-000000000000'::uuid),
    provider, repository, kind, external_id, number, title, url, state, author,
    detected_in, advanced_issue, source_updated_at, merged_at, closed_at,
    created_at, updated_at`

func scanLink(row interface{ Scan(...any) error }) (entity.CodeLink, error) {
	var link entity.CodeLink

	err := row.Scan(
		&link.ID,
		&link.WorkspaceID,
		&link.IssueID,
		&link.ConnectionID,
		&link.Provider,
		&link.Repository,
		&link.Kind,
		&link.ExternalID,
		&link.Number,
		&link.Title,
		&link.URL,
		&link.State,
		&link.Author,
		&link.DetectedIn,
		&link.AdvancedIssue,
		&link.SourceUpdatedAt,
		&link.MergedAt,
		&link.ClosedAt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		return entity.CodeLink{}, err
	}

	return link, nil
}

// upsertLinkQuery refuses an update carrying an older source timestamp than the row already
// holds. Deliveries arrive out of order, so a change reopened after it closed would otherwise
// settle as whichever event happened to be handled last rather than whichever happened last.
const upsertLinkQuery = `
INSERT INTO workspace_code_links (
    id, workspace_id, issue_id, connection_id, provider, repository, kind, external_id,
    number, title, url, state, author, detected_in, source_updated_at, merged_at, closed_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
ON CONFLICT (issue_id, provider, repository, kind, external_id) DO UPDATE
SET connection_id = excluded.connection_id,
    number = excluded.number,
    title = excluded.title,
    url = excluded.url,
    state = excluded.state,
    author = excluded.author,
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
WHERE issue_id = $1 AND provider = $2 AND repository = $3 AND kind = $4 AND external_id = $5`

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
		connectionOrNil(link.ConnectionID),
		link.Provider,
		link.Repository,
		link.Kind,
		link.ExternalID,
		link.Number,
		link.Title,
		link.URL,
		link.State,
		link.Author,
		link.DetectedIn,
		link.SourceUpdatedAt,
		link.MergedAt,
		link.ClosedAt,
	))

	// No row comes back when the guard above refused a stale update. The link is unchanged
	// and still wanted, so read it rather than reporting a failure the caller cannot act on.
	if errors.Is(err, sql.ErrNoRows) {
		return scanLinkOrFail(r.db.Querier(ctx).QueryRowContext(
			ctx,
			readLinkQuery,
			link.IssueID,
			link.Provider,
			link.Repository,
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

func connectionOrNil(connectionID uuid.UUID) any {
	if connectionID == uuid.Nil {
		return nil
	}

	return connectionID
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
WHERE workspace_id = $1 AND provider = $2 AND repository = $3 AND external_id = $4
ORDER BY created_at, id`

func (r *linkRepository) ListByExternalID(
	ctx context.Context,
	workspaceID uuid.UUID,
	provider entity.SCMProvider,
	repo, externalID string,
) ([]entity.CodeLink, error) {
	return r.collect(ctx, listLinksByExternalQuery, workspaceID, provider, repo, externalID)
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

const markAdvancedQuery = `
UPDATE workspace_code_links
SET advanced_issue = true, updated_at = now()
WHERE id = $1`

func (r *linkRepository) MarkAdvanced(ctx context.Context, linkID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, markAdvancedQuery, linkID)
	if err != nil {
		return fmt.Errorf("record that a linked change advanced its issue: %w", err)
	}

	return expectOne(result, entity.ErrCodeLinkNotFound)
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

// detachLinksQuery is what makes disconnecting keep history. Everything a link needs to
// render — the forge, the repository, the number, the title and a working address — is on
// the row already, so cutting it loose from the connection costs the reader nothing.
const detachLinksQuery = `
UPDATE workspace_code_links
SET connection_id = NULL, updated_at = now()
WHERE connection_id = $1`

func (r *linkRepository) DetachConnection(ctx context.Context, connectionID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, detachLinksQuery, connectionID); err != nil {
		return fmt.Errorf("detach linked changes from a connection: %w", err)
	}

	return nil
}

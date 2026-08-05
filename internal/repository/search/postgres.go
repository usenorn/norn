package search

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const seesTeam = `(m.role = 'admin' OR t.visibility = 'public' OR tm.account_id IS NOT NULL)`

const issueInScope = `
  AND ($7::boolean IS TRUE OR i.team_id = ANY($8::uuid[]))`

const teamInScope = `
  AND ($7::boolean IS TRUE OR t.id = ANY($8::uuid[]))`

const fuzzyIssueInScope = `
  AND ($5::boolean IS TRUE OR i.team_id = ANY($6::uuid[]))`

const documentMatches = `
  AND (($3 <> '' AND %[1]s @@ websearch_to_tsquery('english', $3))
       OR ($4 <> '' AND %[1]s @@ to_tsquery('simple', $4 || ':*')))`

const titleWeighted = `(($3 <> '' AND ts_rank_cd(ARRAY[0, 0, 0, 1]::float4[], %[1]s,
                websearch_to_tsquery('english', $3)) > 0)
        OR ($4 <> '' AND ts_rank_cd(ARRAY[0, 0, 0, 1]::float4[], %[1]s,
                to_tsquery('simple', $4 || ':*')) > 0))`

const bounded = `
LIMIT $5
)
SELECT kind, id, issue_id, title, excerpt, reference, team_key, slug, status, title_hit, updated_at
FROM candidates
ORDER BY title_hit DESC, updated_at DESC, id DESC
LIMIT $6`

const issueResults = `
WITH candidates AS (
SELECT 'issue' AS kind,
       i.id::text AS id,
       i.id::text AS issue_id,
       i.title,
       '' AS excerpt,
       i.reference_key || '-' || i.number::text AS reference,
       t.key AS team_key,
       '' AS slug,
       i.status,
       ` + titleWeighted + ` AS title_hit,
       i.updated_at
FROM workspace_issues i
JOIN workspace_memberships m ON m.workspace_id = i.workspace_id AND m.account_id = $2
JOIN workspace_teams t ON t.id = i.team_id AND t.status = 'active'
LEFT JOIN workspace_team_members tm ON tm.team_id = t.id AND tm.account_id = $2
WHERE i.workspace_id = $1
  AND i.status = 'active'
  AND ` + seesTeam + issueInScope + documentMatches + bounded

const commentResults = `
WITH candidates AS (
SELECT 'comment' AS kind,
       c.id::text AS id,
       i.id::text AS issue_id,
       i.title,
       left(c.body, 200) AS excerpt,
       i.reference_key || '-' || i.number::text AS reference,
       t.key AS team_key,
       '' AS slug,
       i.status,
       false AS title_hit,
       c.updated_at
FROM workspace_issue_comments c
JOIN workspace_issues i ON i.id = c.issue_id AND i.status = 'active'
JOIN workspace_memberships m ON m.workspace_id = c.workspace_id AND m.account_id = $2
JOIN workspace_teams t ON t.id = i.team_id AND t.status = 'active'
LEFT JOIN workspace_team_members tm ON tm.team_id = t.id AND tm.account_id = $2
WHERE c.workspace_id = $1
  AND c.deleted_at IS NULL
  AND ` + seesTeam + issueInScope + documentMatches + bounded

const projectResults = `
WITH candidates AS (
SELECT 'project' AS kind,
       p.id::text AS id,
       '' AS issue_id,
       p.name AS title,
       left(p.description, 200) AS excerpt,
       '' AS reference,
       '' AS team_key,
       p.slug,
       p.state AS status,
       ` + titleWeighted + ` AS title_hit,
       p.updated_at
FROM workspace_projects p
JOIN workspace_memberships m ON m.workspace_id = p.workspace_id AND m.account_id = $2
WHERE p.workspace_id = $1
  AND p.archived_at IS NULL` + documentMatches + bounded

const teamResults = `
WITH candidates AS (
SELECT 'team' AS kind,
       t.id::text AS id,
       '' AS issue_id,
       t.name AS title,
       '' AS excerpt,
       '' AS reference,
       t.key AS team_key,
       '' AS slug,
       t.status,
       true AS title_hit,
       t.updated_at
FROM workspace_teams t
JOIN workspace_memberships m ON m.workspace_id = t.workspace_id AND m.account_id = $2
LEFT JOIN workspace_team_members tm ON tm.team_id = t.id AND tm.account_id = $2
WHERE t.workspace_id = $1
  AND t.status = 'active'
  AND ` + seesTeam + teamInScope + `
  AND ($3 <> '' OR $4 <> '')
  AND (position(lower($3) IN lower(t.name)) > 0
       OR position(lower($4) IN lower(t.key)) > 0)` + bounded

const personResults = `
WITH candidates AS (
SELECT 'person' AS kind,
       a.id::text AS id,
       '' AS issue_id,
       a.display_name AS title,
       '' AS excerpt,
       '' AS reference,
       '' AS team_key,
       '' AS slug,
       m.role AS status,
       true AS title_hit,
       m.updated_at
FROM workspace_memberships m
JOIN accounts a
    ON a.id = m.account_id
   AND a.status = 'active'
   AND a.kind <> 'agent'
   AND coalesce(a.display_name, '') <> ''
JOIN workspace_memberships viewer
    ON viewer.workspace_id = m.workspace_id AND viewer.account_id = $2
WHERE m.workspace_id = $1
  AND ($3 <> '' OR $4 <> '')
  AND position(lower($3) IN lower(a.display_name)) > 0` + bounded

const fuzzyIssueResults = `
SELECT 'issue' AS kind,
       i.id::text AS id,
       i.id::text AS issue_id,
       i.title,
       '' AS excerpt,
       i.reference_key || '-' || i.number::text AS reference,
       t.key AS team_key,
       '' AS slug,
       i.status,
       true AS title_hit,
       i.updated_at
FROM workspace_issues i
JOIN workspace_memberships m ON m.workspace_id = i.workspace_id AND m.account_id = $2
JOIN workspace_teams t ON t.id = i.team_id AND t.status = 'active'
LEFT JOIN workspace_team_members tm ON tm.team_id = t.id AND tm.account_id = $2
WHERE i.workspace_id = $1
  AND i.status = 'active'
  AND ` + seesTeam + fuzzyIssueInScope + `
  AND $3 <% i.title
ORDER BY $3 <<-> i.title, i.id
LIMIT $4`

const pinSimilarity = `SET LOCAL pg_trgm.word_similarity_threshold = `

type searchRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Search {
	return &searchRepository{db: db}
}

func statementFor(kind entity.SearchKind) (string, bool) {
	switch kind {
	case entity.SearchKindComment:
		return fmt.Sprintf(commentResults, "c.search_document"), true
	case entity.SearchKindProject:
		return fmt.Sprintf(projectResults, "p.search_document"), false
	case entity.SearchKindTeam:
		return teamResults, true
	case entity.SearchKindPerson:
		return personResults, false
	default:
		return fmt.Sprintf(issueResults, "i.search_document"), true
	}
}

func teamIDs(scope entity.TeamScope) []string {
	ids := make([]string, 0, len(scope.TeamIDs))

	for _, id := range scope.TeamIDs {
		ids = append(ids, id.String())
	}

	return ids
}

func (r *searchRepository) Search(
	ctx context.Context,
	request repository.SearchRequest,
) ([]entity.SearchGroup, error) {
	groups := make([]entity.SearchGroup, 0, len(request.Kinds))

	for _, kind := range request.Kinds {
		if !kind.Valid() {
			continue
		}

		statement, scoped := statementFor(kind)

		args := []any{
			request.WorkspaceID.String(), request.AccountID.String(),
			request.Query.Stemmed, request.Query.Prefix,
			entity.SearchCandidateCap, request.Limit + 1,
		}

		if scoped {
			args = append(args, request.Scope.AllTeams, teamIDs(request.Scope))
		}

		results, err := r.query(ctx, statement, args...)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group(kind, results, request.Limit))
	}

	return groups, nil
}

func (r *searchRepository) Fuzzy(
	ctx context.Context,
	request repository.SearchRequest,
) ([]entity.SearchGroup, error) {
	if !slices.Contains(request.Kinds, entity.SearchKindIssue) || request.Query.Raw == "" {
		return []entity.SearchGroup{}, nil
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, pinSimilarity+entity.SearchSimilarityThreshold,
	); err != nil {
		return nil, fmt.Errorf("pin similarity threshold: %w", err)
	}

	results, err := r.query(
		ctx, fuzzyIssueResults,
		request.WorkspaceID.String(), request.AccountID.String(),
		request.Query.Raw, request.Limit+1,
		request.Scope.AllTeams, teamIDs(request.Scope),
	)
	if err != nil {
		return nil, err
	}

	return []entity.SearchGroup{group(entity.SearchKindIssue, results, request.Limit)}, nil
}

func group(kind entity.SearchKind, results []entity.SearchResult, limit int) entity.SearchGroup {
	found := entity.SearchGroup{Kind: kind, Results: results}

	if len(results) > limit {
		found.Results = results[:limit]
		found.More = true
	}

	return found
}

func (r *searchRepository) query(
	ctx context.Context,
	statement string,
	args ...any,
) ([]entity.SearchResult, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("read search results: %w", err)
	}

	defer func() { _ = rows.Close() }()

	results := make([]entity.SearchResult, 0)

	for rows.Next() {
		var (
			result           entity.SearchResult
			kind, id, issue  string
			excerpt          string
			slug, status     string
			updatedAt        time.Time
			reference, teamK string
		)

		if err := rows.Scan(
			&kind, &id, &issue, &result.Title, &excerpt, &reference,
			&teamK, &slug, &status, &result.TitleHit, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}

		result.Kind = entity.SearchKind(kind)
		result.ID = parse(id)
		result.IssueID = parse(issue)
		result.Excerpt = strings.TrimSpace(excerpt)
		result.Reference = reference
		result.TeamKey = teamK
		result.Slug = slug
		result.Status = status
		result.UpdatedAt = updatedAt

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read search results: %w", err)
	}

	return results, nil
}

func parse(raw string) uuid.UUID {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}

	return parsed
}

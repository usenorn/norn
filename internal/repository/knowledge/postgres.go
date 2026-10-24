package knowledge

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const queryTerms = `
SELECT (CASE WHEN $%[1]d <> '' THEN replace(plainto_tsquery('english', $%[1]d)::text, ' & ', ' | ')::tsquery
             ELSE ''::tsquery END ||
        CASE WHEN $%[2]d <> '' THEN to_tsquery('simple', $%[2]d || ':*') ELSE ''::tsquery END) AS terms`

const memoryFilters = `
WHERE m.workspace_id = $1
  AND (m.visibility = 'workspace'
       OR (m.visibility = 'project' AND ($2::boolean IS TRUE OR m.project_id = ANY($3::uuid[])))
       OR (m.visibility = 'personal' AND m.author_account_id = $4::uuid))
  AND m.status = ANY($5::text[])
  AND (cardinality($6::text[]) = 0 OR m.kind = ANY($6::text[]))
  AND m.effective_from <= $7::date
  AND (m.effective_until IS NULL OR m.effective_until >= $7::date)
  AND ($8::uuid IS NULL OR m.applies_project_id IS NULL OR m.applies_project_id = $8::uuid)
  AND ($9::text = '' OR m.component = '' OR m.component = $9::text)`

const passageSource = `
FROM workspace_doc_passages p
JOIN workspace_docs d ON d.id = p.doc_id AND d.workspace_id = p.workspace_id AND d.version = p.doc_version`

const passageFilters = `
WHERE p.workspace_id = $1
  AND d.status = 'active'
  AND (d.visibility = 'workspace'
       OR (d.visibility = 'project' AND ($2::boolean IS TRUE OR d.project_id = ANY($3::uuid[])))
       OR (d.visibility = 'personal' AND d.author_account_id = $4::uuid))
  AND ($5::uuid IS NULL OR d.project_id IS NULL OR d.project_id = $5::uuid)`

const passageColumns = `
       p.id, d.id AS doc_id, d.title, p.heading, p.body, p.position,
       d.visibility, coalesce(d.project_id::text, '') AS project_id,
       coalesce(d.author_account_id::text, '') AS author_id, d.updated_at`

const passageExcerpt = `StartSel="", StopSel="", MaxFragments=2, MaxWords=32, MinWords=12, FragmentDelimiter=" … "`

var lexicalMemoriesQuery = `
WITH q AS (` + fmt.Sprintf(queryTerms, 11, 12) + `),
candidates AS (
SELECT m.id,
       (lower(m.statement) = lower($10) OR m.id::text = lower($10)) AS exact,
       ts_rank_cd(m.search_document, q.terms) AS rank,
       m.updated_at
FROM workspace_memories m, q` + memoryFilters + `
  AND (lower(m.statement) = lower($10) OR m.id::text = lower($10)
       OR (numnode(q.terms) > 0 AND m.search_document @@ q.terms))
LIMIT $13
)
SELECT id, exact
FROM candidates
ORDER BY exact DESC, rank DESC, updated_at DESC, id
LIMIT $14`

var semanticMemoriesQuery = `
SELECT m.id
FROM workspace_memories m
JOIN workspace_knowledge_embeddings e
  ON e.workspace_id = m.workspace_id AND e.content_hash = m.content_hash AND e.model = $10` + memoryFilters + `
  AND 1 - (e.embedding <=> $11::vector) >= $12
ORDER BY e.embedding <=> $11::vector, m.id
LIMIT $13`

var lexicalPassagesQuery = `
WITH q AS (` + fmt.Sprintf(queryTerms, 7, 8) + `),
candidates AS (
SELECT` + passageColumns + `,
       (p.position = 0 AND (lower(d.title) = lower($6) OR d.id::text = lower($6))) AS exact,
       ts_rank_cd(p.search_document, q.terms) AS rank` + passageSource + `
CROSS JOIN q` + passageFilters + `
  AND ((p.position = 0 AND (lower(d.title) = lower($6) OR d.id::text = lower($6)))
       OR (numnode(q.terms) > 0 AND p.search_document @@ q.terms))
LIMIT $9
)
SELECT c.id, c.doc_id, c.title, c.heading,
       CASE WHEN c.body = '' THEN '' ELSE ts_headline('english', c.body, q.terms, '` + passageExcerpt + `') END,
       c.position, c.visibility, c.project_id, c.author_id, c.updated_at, c.exact
FROM candidates c, q
ORDER BY c.exact DESC, c.rank DESC, c.updated_at DESC, c.id
LIMIT $10`

var semanticPassagesQuery = `
SELECT` + passageColumns + passageSource + `
JOIN workspace_knowledge_embeddings e
  ON e.workspace_id = p.workspace_id AND e.content_hash = p.content_hash AND e.model = $6` + passageFilters + `
  AND 1 - (e.embedding <=> $7::vector) >= $8
ORDER BY e.embedding <=> $7::vector, p.id
LIMIT $9`

type knowledgeRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Knowledge {
	return &knowledgeRepository{db: db}
}

func memoryArgs(workspaceID uuid.UUID, query repository.KnowledgeQuery) []any {
	statuses := make([]string, len(query.Statuses))
	for index, status := range query.Statuses {
		statuses[index] = string(status)
	}

	kinds := make([]string, len(query.MemoryKinds))
	for index, kind := range query.MemoryKinds {
		kinds[index] = string(kind)
	}

	return []any{
		workspaceID,
		query.AllProjects,
		idStrings(query.ProjectIDs),
		nullable(query.PersonalAuthorID),
		statuses,
		kinds,
		query.On,
		optional(query.ProjectID),
		query.Component,
	}
}

func passageArgs(workspaceID uuid.UUID, query repository.KnowledgeQuery) []any {
	return []any{
		workspaceID,
		query.AllProjects,
		idStrings(query.ProjectIDs),
		nullable(query.PersonalAuthorID),
		optional(query.ProjectID),
	}
}

func idStrings(ids []uuid.UUID) []string {
	strs := make([]string, len(ids))
	for index, id := range ids {
		strs[index] = id.String()
	}

	return strs
}

func nullable(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}

	return id
}

func optional(id *uuid.UUID) any {
	if id == nil {
		return nil
	}

	return *id
}

func (r *knowledgeRepository) LexicalMemories(
	ctx context.Context,
	workspaceID uuid.UUID,
	query repository.KnowledgeQuery,
) ([]entity.KnowledgeCandidate, error) {
	args := append(memoryArgs(workspaceID, query),
		query.Query.Raw, query.Query.Stemmed, query.Query.Prefix, entity.SearchCandidateCap, query.Limit)

	return r.memories(ctx, "match memories by their words", lexicalMemoriesQuery, true, args)
}

func (r *knowledgeRepository) SemanticMemories(
	ctx context.Context,
	workspaceID uuid.UUID,
	query repository.KnowledgeQuery,
	vector repository.KnowledgeVector,
) ([]entity.KnowledgeCandidate, error) {
	args := append(memoryArgs(workspaceID, query),
		vector.Model, vectorLiteral(vector.Vector), vector.MinSimilarity, query.Limit)

	return r.memories(ctx, "match memories by meaning", semanticMemoriesQuery, false, args)
}

func (r *knowledgeRepository) memories(
	ctx context.Context,
	action, statement string,
	withExact bool,
	args []any,
) ([]entity.KnowledgeCandidate, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	defer func() { _ = rows.Close() }()

	var candidates []entity.KnowledgeCandidate

	for rows.Next() {
		candidate := entity.KnowledgeCandidate{Kind: entity.KnowledgeSubjectMemory}

		targets := []any{&candidate.ID}
		if withExact {
			targets = append(targets, &candidate.Exact)
		}

		if err := rows.Scan(targets...); err != nil {
			return nil, fmt.Errorf("scan %s: %w", action, err)
		}

		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", action, err)
	}

	return candidates, nil
}

func (r *knowledgeRepository) LexicalPassages(
	ctx context.Context,
	workspaceID uuid.UUID,
	query repository.KnowledgeQuery,
) ([]entity.KnowledgeCandidate, error) {
	args := append(passageArgs(workspaceID, query),
		query.Query.Raw, query.Query.Stemmed, query.Query.Prefix, entity.SearchCandidateCap, query.Limit)

	rows, err := r.db.Querier(ctx).QueryContext(ctx, lexicalPassagesQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("match doc passages by their words: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var candidates []entity.KnowledgeCandidate

	for rows.Next() {
		var (
			passage entity.KnowledgePassage
			raw     passageRow
			exact   bool
		)

		if err := rows.Scan(
			&passage.ID, &passage.DocID, &passage.DocTitle, &passage.Heading, &passage.Excerpt, &passage.Position,
			&raw.visibility, &raw.projectID, &raw.authorID, &passage.UpdatedAt, &exact,
		); err != nil {
			return nil, fmt.Errorf("scan doc passage match: %w", err)
		}

		candidate, err := raw.candidate(workspaceID, passage, exact)
		if err != nil {
			return nil, err
		}

		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read doc passage matches: %w", err)
	}

	return candidates, nil
}

func (r *knowledgeRepository) SemanticPassages(
	ctx context.Context,
	workspaceID uuid.UUID,
	query repository.KnowledgeQuery,
	vector repository.KnowledgeVector,
) ([]entity.KnowledgeCandidate, error) {
	args := append(passageArgs(workspaceID, query),
		vector.Model, vectorLiteral(vector.Vector), vector.MinSimilarity, query.Limit)

	rows, err := r.db.Querier(ctx).QueryContext(ctx, semanticPassagesQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("match doc passages by meaning: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var candidates []entity.KnowledgeCandidate

	for rows.Next() {
		var (
			passage entity.KnowledgePassage
			raw     passageRow
			body    string
		)

		if err := rows.Scan(
			&passage.ID, &passage.DocID, &passage.DocTitle, &passage.Heading, &body, &passage.Position,
			&raw.visibility, &raw.projectID, &raw.authorID, &passage.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan doc passage match: %w", err)
		}

		passage.Excerpt = excerpt(body)

		candidate, err := raw.candidate(workspaceID, passage, false)
		if err != nil {
			return nil, err
		}

		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read doc passage matches: %w", err)
	}

	return candidates, nil
}

type passageRow struct {
	visibility string
	projectID  string
	authorID   string
}

func (r passageRow) candidate(
	workspaceID uuid.UUID,
	passage entity.KnowledgePassage,
	exact bool,
) (entity.KnowledgeCandidate, error) {
	passage.WorkspaceID = workspaceID
	passage.Audience.Visibility = entity.KnowledgeVisibility(r.visibility)

	if r.projectID != "" {
		projectID, err := uuid.Parse(r.projectID)
		if err != nil {
			return entity.KnowledgeCandidate{}, fmt.Errorf("parse doc project id: %w", err)
		}

		passage.Audience.ProjectID = projectID
	}

	if r.authorID != "" {
		authorID, err := uuid.Parse(r.authorID)
		if err != nil {
			return entity.KnowledgeCandidate{}, fmt.Errorf("parse doc author id: %w", err)
		}

		passage.AuthorID = authorID
	}

	return entity.KnowledgeCandidate{
		Kind:    entity.KnowledgeSubjectDocPassage,
		ID:      passage.ID,
		Exact:   exact,
		Passage: &passage,
	}, nil
}

func excerpt(body string) string {
	runes := []rune(body)
	if len(runes) <= entity.KnowledgeExcerptRunes {
		return body
	}

	return string(runes[:entity.KnowledgeExcerptRunes]) + "…"
}

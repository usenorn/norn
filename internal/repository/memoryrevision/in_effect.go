package memoryrevision

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

const inEffectQuery = `
WITH latest AS (
    SELECT DISTINCT ON (r.memory_id) r.*
    FROM workspace_memory_revisions r
    WHERE r.workspace_id = $1 AND r.created_at <= $5
    ORDER BY r.memory_id, r.version DESC
),
q AS (
    SELECT (CASE WHEN $11 <> '' THEN replace(plainto_tsquery('english', $11)::text, ' & ', ' | ')::tsquery
                 ELSE ''::tsquery END ||
            CASE WHEN $12 <> '' THEN to_tsquery('simple', $12 || ':*') ELSE ''::tsquery END) AS terms
),
candidates AS (
    SELECT l.*,
           m.visibility AS current_visibility,
           m.project_id AS current_project_id,
           m.author_account_id AS author_id,
           m.supersedes_id AS current_supersedes_id,
           m.created_at AS memory_created_at,
           setweight(to_tsvector('english', l.statement), 'A') ||
           setweight(to_tsvector('simple', l.statement), 'A') ||
           setweight(to_tsvector('english', l.rationale || ' ' || l.conditions), 'B') ||
           setweight(to_tsvector('simple', l.rationale || ' ' || l.conditions), 'B') AS document
    FROM latest l
    JOIN workspace_memories m ON m.id = l.memory_id AND m.workspace_id = l.workspace_id
    WHERE (m.visibility = 'workspace'
           OR (m.visibility = 'project' AND ($2::boolean IS TRUE OR m.project_id = ANY($3::uuid[])))
           OR (m.visibility = 'personal' AND m.author_account_id = $4::uuid))
      AND l.status = ANY($6::text[])
      AND (cardinality($7::text[]) = 0 OR l.kind = ANY($7::text[]))
      AND l.effective_from <= $5::date
      AND (l.effective_until IS NULL OR l.effective_until >= $5::date)
      AND ($8::uuid IS NULL OR l.applies_project_id IS NULL OR l.applies_project_id = $8::uuid)
      AND ($9::text = '' OR l.component = '' OR l.component = $9::text)
)
SELECT c.memory_id,
       c.version,
       c.kind,
       c.statement,
       c.rationale,
       c.conditions,
       c.current_visibility,
       coalesce(c.current_project_id::text, ''),
       coalesce(c.applies_project_id::text, ''),
       c.component,
       c.status,
       c.basis,
       coalesce(c.author_id::text, ''),
       c.effective_from,
       c.effective_until,
       coalesce(c.current_supersedes_id::text, ''),
       c.memory_created_at,
       c.created_at,
       (lower(c.statement) = lower($10) OR c.memory_id::text = lower($10)) AS exact
FROM candidates c, q
WHERE lower(c.statement) = lower($10) OR c.memory_id::text = lower($10)
   OR (numnode(q.terms) > 0 AND c.document @@ q.terms)
ORDER BY exact DESC, ts_rank_cd(c.document, q.terms) DESC, c.memory_id
LIMIT $13`

func (r *revisionRepository) InEffect(
	ctx context.Context,
	workspaceID uuid.UUID,
	query repository.KnowledgeQuery,
) ([]entity.KnowledgeCandidate, error) {
	statuses := make([]string, len(query.Statuses))
	for index, status := range query.Statuses {
		statuses[index] = string(status)
	}

	kinds := make([]string, len(query.MemoryKinds))
	for index, kind := range query.MemoryKinds {
		kinds[index] = string(kind)
	}

	projectIDs := make([]string, len(query.ProjectIDs))
	for index, projectID := range query.ProjectIDs {
		projectIDs[index] = projectID.String()
	}

	var applies sql.NullString
	if query.ProjectID != nil {
		applies = sql.NullString{String: query.ProjectID.String(), Valid: true}
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, inEffectQuery,
		workspaceID.String(),
		query.AllProjects,
		projectIDs,
		sql.NullString{String: query.PersonalAuthorID.String(), Valid: query.PersonalAuthorID != uuid.Nil},
		query.On,
		statuses,
		kinds,
		applies,
		query.Component,
		query.Query.Raw,
		query.Query.Stemmed,
		query.Query.Prefix,
		query.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list memories in effect: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var candidates []entity.KnowledgeCandidate

	for rows.Next() {
		memory, exact, err := scanInEffect(rows, workspaceID)
		if err != nil {
			return nil, err
		}

		candidates = append(candidates, entity.KnowledgeCandidate{
			Kind:   entity.KnowledgeSubjectMemory,
			ID:     memory.ID,
			Exact:  exact,
			Memory: &memory,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read memories in effect: %w", err)
	}

	return candidates, nil
}

func scanInEffect(rows *sql.Rows, workspaceID uuid.UUID) (entity.Memory, bool, error) {
	var (
		memory                                                entity.Memory
		id, kind, visibility, project, applies, status, basis string
		author, supersedes                                    string
		effectiveUntil                                        sql.NullTime
		exact                                                 bool
	)

	if err := rows.Scan(
		&id, &memory.Version, &kind, &memory.Statement, &memory.Rationale, &memory.Conditions, &visibility,
		&project, &applies, &memory.Applicability.Component, &status, &basis, &author, &memory.EffectiveFrom,
		&effectiveUntil, &supersedes, &memory.CreatedAt, &memory.UpdatedAt, &exact,
	); err != nil {
		return entity.Memory{}, false, fmt.Errorf("scan memory in effect: %w", err)
	}

	var err error

	if memory.ID, err = uuid.Parse(id); err != nil {
		return entity.Memory{}, false, fmt.Errorf("parse memory id: %w", err)
	}

	projectID, err := parseOrNil(project)
	if err != nil {
		return entity.Memory{}, false, fmt.Errorf("parse memory project id: %w", err)
	}

	if memory.Applicability.ProjectID, err = parseOptional(applies); err != nil {
		return entity.Memory{}, false, fmt.Errorf("parse memory applicability project id: %w", err)
	}

	if memory.AuthorID, err = parseOrNil(author); err != nil {
		return entity.Memory{}, false, fmt.Errorf("parse memory author id: %w", err)
	}

	if memory.SupersedesID, err = parseOptional(supersedes); err != nil {
		return entity.Memory{}, false, fmt.Errorf("parse superseded memory id: %w", err)
	}

	memory.WorkspaceID = workspaceID
	memory.Kind = entity.MemoryKind(kind)
	memory.Status = entity.MemoryStatus(status)
	memory.Basis = entity.MemoryBasis(basis)
	memory.Audience = entity.KnowledgeAudience{Visibility: entity.KnowledgeVisibility(visibility), ProjectID: projectID}
	memory.EffectiveFrom = entity.MemoryDay(memory.EffectiveFrom)
	memory.UpdatedAt = memory.UpdatedAt.UTC()

	if effectiveUntil.Valid {
		until := entity.MemoryDay(effectiveUntil.Time)
		memory.EffectiveUntil = &until
	}

	return memory, exact, nil
}

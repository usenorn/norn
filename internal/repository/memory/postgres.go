package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const memoryColumns = `
       m.id,
       m.workspace_id,
       m.kind,
       m.statement,
       m.rationale,
       m.conditions,
       m.visibility,
       coalesce(m.project_id::text, ''),
       coalesce(m.applies_project_id::text, ''),
       m.component,
       m.status,
       m.basis,
       coalesce(m.author_account_id::text, ''),
       coalesce(a.display_name, ''),
       coalesce(m.agent_id::text, ''),
       coalesce(m.connection_id::text, ''),
       m.source,
       m.effective_from,
       m.effective_until,
       coalesce(m.supersedes_id::text, ''),
       coalesce(m.superseded_by_id::text, ''),
       m.version,
       m.created_at,
       m.updated_at,
       coalesce((SELECT string_agg(DISTINCT r.account_id::text, ',')
                 FROM workspace_memory_revisions r
                 WHERE r.memory_id = m.id AND r.account_id IS NOT NULL), '')`

const memoryByIDQuery = `
SELECT` + memoryColumns + `
FROM workspace_memories m
LEFT JOIN accounts a ON a.id = m.author_account_id
WHERE m.id = $1 AND m.workspace_id = $2`

const lockMemoryQuery = memoryByIDQuery + `
FOR UPDATE OF m`

const visibleMemories = `
FROM workspace_memories m
LEFT JOIN accounts a ON a.id = m.author_account_id
WHERE m.workspace_id = $1
  AND (m.visibility = 'workspace'
       OR (m.visibility = 'project' AND ($2::boolean IS TRUE OR m.project_id = ANY($3::uuid[])))
       OR (m.visibility = 'personal' AND m.author_account_id = $4::uuid))`

const filteredMemories = `
  AND (cardinality($%[1]d::text[]) = 0 OR m.kind = ANY($%[1]d::text[]))
  AND (cardinality($%[2]d::text[]) = 0 OR m.status = ANY($%[2]d::text[]))
  AND ($%[3]d::uuid IS NULL OR m.applies_project_id = $%[3]d::uuid
       OR ($%[5]d::boolean AND m.applies_project_id IS NULL))
  AND ($%[4]d::text = '' OR m.component = $%[4]d::text OR ($%[5]d::boolean AND m.component = ''))
  AND ($%[6]d::date IS NULL
       OR (m.effective_from <= $%[6]d::date AND (m.effective_until IS NULL OR m.effective_until >= $%[6]d::date)))`

var listMemoriesQuery = `
SELECT` + memoryColumns + visibleMemories + `
  AND ($5::timestamptz IS NULL OR (m.updated_at, m.id) < ($5::timestamptz, $6::uuid))` +
	fmt.Sprintf(filteredMemories, 8, 9, 10, 11, 12, 13) + `
ORDER BY m.updated_at DESC, m.id DESC
LIMIT $7`

var enumerateMemoriesQuery = `
SELECT` + memoryColumns + visibleMemories + `
  AND ($5::uuid IS NULL OR m.id > $5::uuid)` +
	fmt.Sprintf(filteredMemories, 7, 8, 9, 10, 11, 12) + `
ORDER BY m.id
LIMIT $6`

const memoriesByIDsQuery = `
SELECT` + memoryColumns + `
FROM workspace_memories m
LEFT JOIN accounts a ON a.id = m.author_account_id
WHERE m.workspace_id = $1 AND m.id = ANY($2::uuid[])`

const evidenceQuery = `
SELECT e.memory_id,
       e.kind,
       coalesce(e.issue_id::text, ''),
       coalesce(e.comment_id::text, ''),
       coalesce(e.doc_id::text, ''),
       coalesce(e.doc_revision_id::text, ''),
       coalesce(e.excerpt, ''),
       coalesce(d.visibility, ''),
       coalesce(d.project_id::text, ''),
       coalesce(d.author_account_id::text, '')
FROM workspace_memory_evidence e
LEFT JOIN workspace_docs d ON d.id = e.doc_id
WHERE e.memory_id = ANY($1::uuid[])
ORDER BY e.memory_id, e.position`

func memoryToModel(memory entity.Memory) *dbpostgres.WorkspaceMemory {
	return &dbpostgres.WorkspaceMemory{
		ID:               memory.ID.String(),
		WorkspaceID:      memory.WorkspaceID.String(),
		Kind:             string(memory.Kind),
		Statement:        memory.Statement,
		Rationale:        memory.Rationale,
		Conditions:       memory.Conditions,
		Visibility:       string(memory.Audience.Visibility),
		ProjectID:        audienceProject(memory.Audience),
		AppliesProjectID: optionalUUID(memory.Applicability.ProjectID),
		Component:        memory.Applicability.Component,
		Status:           string(memory.Status),
		Basis:            string(memory.Basis),
		AuthorAccountID:  null.NewString(memory.AuthorID.String(), memory.AuthorID != uuid.Nil),
		AgentID:          optionalUUID(memory.By.AgentID),
		ConnectionID:     optionalUUID(memory.By.ConnectionID),
		Source:           string(memory.By.Source),
		EffectiveFrom:    memory.EffectiveFrom,
		EffectiveUntil:   null.TimeFromPtr(memory.EffectiveUntil),
		SupersedesID:     optionalUUID(memory.SupersedesID),
		SupersededByID:   optionalUUID(memory.SupersededByID),
		Version:          memory.Version,
		CreatedAt:        memory.CreatedAt,
		UpdatedAt:        memory.UpdatedAt,
	}
}

func evidenceToModel(memory entity.Memory, position int, evidence entity.MemoryEvidence) *dbpostgres.WorkspaceMemoryEvidence {
	return &dbpostgres.WorkspaceMemoryEvidence{
		ID:            uuid.NewString(),
		WorkspaceID:   memory.WorkspaceID.String(),
		MemoryID:      memory.ID.String(),
		Position:      position,
		Kind:          string(evidence.Kind),
		IssueID:       optionalUUID(evidence.IssueID),
		CommentID:     optionalUUID(evidence.CommentID),
		DocID:         optionalUUID(evidence.DocID),
		DocRevisionID: optionalUUID(evidence.DocRevisionID),
		Excerpt:       null.NewString(evidence.Excerpt, evidence.Kind == entity.MemoryEvidenceExcerpt),
	}
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

type memoryRow struct {
	id, workspace, kind, visibility, project, appliesProject, status, basis   string
	author, agent, connection, source, supersedes, supersededBy, contributors string
	effectiveUntil                                                            sql.NullTime
}

func scanMemory(row scanner) (entity.Memory, error) {
	var (
		memory entity.Memory
		raw    memoryRow
	)

	if err := row.Scan(
		&raw.id, &raw.workspace, &raw.kind, &memory.Statement, &memory.Rationale, &memory.Conditions,
		&raw.visibility, &raw.project, &raw.appliesProject, &memory.Applicability.Component, &raw.status,
		&raw.basis, &raw.author, &memory.AuthorName, &raw.agent, &raw.connection, &raw.source,
		&memory.EffectiveFrom, &raw.effectiveUntil, &raw.supersedes, &raw.supersededBy, &memory.Version,
		&memory.CreatedAt, &memory.UpdatedAt, &raw.contributors,
	); err != nil {
		return entity.Memory{}, err
	}

	if err := raw.fill(&memory); err != nil {
		return entity.Memory{}, err
	}

	return memory, nil
}

func (row *memoryRow) fill(memory *entity.Memory) error {
	var err error

	if memory.ID, err = uuid.Parse(row.id); err != nil {
		return fmt.Errorf("parse memory id: %w", err)
	}

	if memory.WorkspaceID, err = uuid.Parse(row.workspace); err != nil {
		return fmt.Errorf("parse memory workspace id: %w", err)
	}

	projectID, err := parseOrNil(row.project)
	if err != nil {
		return fmt.Errorf("parse memory project id: %w", err)
	}

	if memory.Applicability.ProjectID, err = parseOptional(row.appliesProject); err != nil {
		return fmt.Errorf("parse memory applicability project id: %w", err)
	}

	if memory.AuthorID, err = parseOrNil(row.author); err != nil {
		return fmt.Errorf("parse memory author id: %w", err)
	}

	if memory.By.AgentID, err = parseOptional(row.agent); err != nil {
		return fmt.Errorf("parse memory agent id: %w", err)
	}

	if memory.By.ConnectionID, err = parseOptional(row.connection); err != nil {
		return fmt.Errorf("parse memory connection id: %w", err)
	}

	if memory.SupersedesID, err = parseOptional(row.supersedes); err != nil {
		return fmt.Errorf("parse superseded memory id: %w", err)
	}

	if memory.SupersededByID, err = parseOptional(row.supersededBy); err != nil {
		return fmt.Errorf("parse replacing memory id: %w", err)
	}

	if memory.ContributorIDs, err = parseList(row.contributors); err != nil {
		return fmt.Errorf("parse memory contributor id: %w", err)
	}

	memory.Kind = entity.MemoryKind(row.kind)
	memory.Audience = entity.KnowledgeAudience{Visibility: entity.KnowledgeVisibility(row.visibility), ProjectID: projectID}
	memory.Status = entity.MemoryStatus(row.status)
	memory.Basis = entity.MemoryBasis(row.basis)
	memory.By.AccountID = memory.AuthorID
	memory.By.Source = entity.RevisionSource(row.source)
	memory.EffectiveFrom = entity.MemoryDay(memory.EffectiveFrom)
	memory.CreatedAt = memory.CreatedAt.UTC()
	memory.UpdatedAt = memory.UpdatedAt.UTC()

	if row.effectiveUntil.Valid {
		until := entity.MemoryDay(row.effectiveUntil.Time)
		memory.EffectiveUntil = &until
	}

	return nil
}

func scanEvidence(row scanner) (uuid.UUID, entity.MemoryEvidence, error) {
	var (
		evidence                                    entity.MemoryEvidence
		memory, kind, issue, comment, doc, revision string
		docVisibility, docProject, docAuthor        string
	)

	if err := row.Scan(
		&memory, &kind, &issue, &comment, &doc, &revision, &evidence.Excerpt,
		&docVisibility, &docProject, &docAuthor,
	); err != nil {
		return uuid.Nil, entity.MemoryEvidence{}, err
	}

	memoryID, err := uuid.Parse(memory)
	if err != nil {
		return uuid.Nil, entity.MemoryEvidence{}, fmt.Errorf("parse evidence memory id: %w", err)
	}

	evidence.Kind = entity.MemoryEvidenceKind(kind)

	for _, target := range []struct {
		raw string
		id  **uuid.UUID
	}{
		{issue, &evidence.IssueID},
		{comment, &evidence.CommentID},
		{doc, &evidence.DocID},
		{revision, &evidence.DocRevisionID},
	} {
		if *target.id, err = parseOptional(target.raw); err != nil {
			return uuid.Nil, entity.MemoryEvidence{}, fmt.Errorf("parse evidence target id: %w", err)
		}
	}

	docProjectID, err := parseOrNil(docProject)
	if err != nil {
		return uuid.Nil, entity.MemoryEvidence{}, fmt.Errorf("parse evidence doc project id: %w", err)
	}

	if evidence.DocAuthorID, err = parseOrNil(docAuthor); err != nil {
		return uuid.Nil, entity.MemoryEvidence{}, fmt.Errorf("parse evidence doc author id: %w", err)
	}

	evidence.DocAudience = entity.KnowledgeAudience{
		Visibility: entity.KnowledgeVisibility(docVisibility),
		ProjectID:  docProjectID,
	}

	return memoryID, evidence, nil
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

func parseList(raw string) ([]uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}

	var ids []uuid.UUID

	for _, part := range strings.Split(raw, ",") {
		id, err := uuid.Parse(part)
		if err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

type memoryRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Memory {
	return &memoryRepository{db: db}
}

func (r *memoryRepository) Create(ctx context.Context, memory entity.Memory) error {
	if err := memoryToModel(memory).Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}

	return r.writeEvidence(ctx, memory)
}

func (r *memoryRepository) writeEvidence(ctx context.Context, memory entity.Memory) error {
	for position, evidence := range memory.Evidence {
		if err := evidenceToModel(memory, position, evidence).Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
			return fmt.Errorf("insert memory evidence: %w", err)
		}
	}

	return nil
}

func (r *memoryRepository) GetByID(ctx context.Context, workspaceID, memoryID uuid.UUID) (entity.Memory, error) {
	return r.find(ctx, memoryByIDQuery, workspaceID, memoryID)
}

func (r *memoryRepository) LockByID(ctx context.Context, workspaceID, memoryID uuid.UUID) (entity.Memory, error) {
	return r.find(ctx, lockMemoryQuery, workspaceID, memoryID)
}

func (r *memoryRepository) find(
	ctx context.Context,
	query string,
	workspaceID, memoryID uuid.UUID,
) (entity.Memory, error) {
	memory, err := scanMemory(r.db.Querier(ctx).QueryRowContext(ctx, query, memoryID.String(), workspaceID.String()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Memory{}, entity.ErrMemoryNotFound
		}

		return entity.Memory{}, fmt.Errorf("find memory: %w", err)
	}

	memories := []entity.Memory{memory}
	if err := r.attachEvidence(ctx, memories); err != nil {
		return entity.Memory{}, err
	}

	return memories[0], nil
}

func (r *memoryRepository) Update(ctx context.Context, memory entity.Memory, expectedVersion int) error {
	model := memoryToModel(memory)

	updated, err := dbpostgres.WorkspaceMemories(
		dbpostgres.WorkspaceMemoryWhere.ID.EQ(model.ID),
		dbpostgres.WorkspaceMemoryWhere.WorkspaceID.EQ(model.WorkspaceID),
		dbpostgres.WorkspaceMemoryWhere.Version.EQ(expectedVersion),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceMemoryColumns.Kind:             model.Kind,
		dbpostgres.WorkspaceMemoryColumns.Statement:        model.Statement,
		dbpostgres.WorkspaceMemoryColumns.Rationale:        model.Rationale,
		dbpostgres.WorkspaceMemoryColumns.Conditions:       model.Conditions,
		dbpostgres.WorkspaceMemoryColumns.Visibility:       model.Visibility,
		dbpostgres.WorkspaceMemoryColumns.ProjectID:        model.ProjectID,
		dbpostgres.WorkspaceMemoryColumns.AppliesProjectID: model.AppliesProjectID,
		dbpostgres.WorkspaceMemoryColumns.Component:        model.Component,
		dbpostgres.WorkspaceMemoryColumns.Status:           model.Status,
		dbpostgres.WorkspaceMemoryColumns.Basis:            model.Basis,
		dbpostgres.WorkspaceMemoryColumns.EffectiveFrom:    model.EffectiveFrom,
		dbpostgres.WorkspaceMemoryColumns.EffectiveUntil:   model.EffectiveUntil,
		dbpostgres.WorkspaceMemoryColumns.SupersededByID:   model.SupersededByID,
		dbpostgres.WorkspaceMemoryColumns.Version:          model.Version,
		dbpostgres.WorkspaceMemoryColumns.UpdatedAt:        model.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("update memory: %w", err)
	}

	if updated == 0 {
		return entity.ErrMemoryStale
	}

	if _, err := dbpostgres.WorkspaceMemoryEvidences(
		dbpostgres.WorkspaceMemoryEvidenceWhere.MemoryID.EQ(model.ID),
	).DeleteAll(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("clear memory evidence: %w", err)
	}

	return r.writeEvidence(ctx, memory)
}

func (r *memoryRepository) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	filter repository.MemoryFilter,
	page entity.MemoryPage,
) ([]entity.Memory, error) {
	var (
		after   sql.NullTime
		afterID sql.NullString
	)

	if page.Cursor != nil {
		after = sql.NullTime{Time: page.Cursor.UpdatedAt, Valid: true}
		afterID = sql.NullString{String: page.Cursor.MemoryID.String(), Valid: true}
	}

	args := append(visibleArgs(workspaceID, filter), after, afterID, page.Limit)

	memories, err := r.query(ctx, "list memories", listMemoriesQuery, append(args, filterArgs(filter)...)...)
	if err != nil {
		return nil, err
	}

	if err := r.attachEvidence(ctx, memories); err != nil {
		return nil, err
	}

	return memories, nil
}

func (r *memoryRepository) ListByIDs(
	ctx context.Context,
	workspaceID uuid.UUID,
	memoryIDs []uuid.UUID,
) ([]entity.Memory, error) {
	if len(memoryIDs) == 0 {
		return nil, nil
	}

	return r.query(ctx, "list memories by id", memoriesByIDsQuery, workspaceID.String(), idStrings(memoryIDs))
}

func (r *memoryRepository) Enumerate(
	ctx context.Context,
	workspaceID uuid.UUID,
	filter repository.MemoryFilter,
	after *uuid.UUID,
	limit int,
) ([]entity.Memory, error) {
	args := append(visibleArgs(workspaceID, filter), nullableID(after), limit)

	return r.query(ctx, "enumerate memories", enumerateMemoriesQuery, append(args, filterArgs(filter)...)...)
}

func visibleArgs(workspaceID uuid.UUID, filter repository.MemoryFilter) []any {
	return []any{
		workspaceID.String(),
		filter.AllProjects,
		idStrings(filter.ProjectIDs),
		nullableAuthor(filter.PersonalAuthorID),
	}
}

func filterArgs(filter repository.MemoryFilter) []any {
	kinds := make([]string, 0, len(filter.Kinds))
	for _, kind := range filter.Kinds {
		kinds = append(kinds, string(kind))
	}

	statuses := make([]string, 0, len(filter.Statuses))
	for _, status := range filter.Statuses {
		statuses = append(statuses, string(status))
	}

	var on sql.NullTime
	if filter.InEffectOn != nil {
		on = sql.NullTime{Time: *filter.InEffectOn, Valid: true}
	}

	return []any{
		kinds,
		statuses,
		nullableID(filter.AppliesProjectID),
		filter.Component,
		filter.WorkspaceWide,
		on,
	}
}

func (r *memoryRepository) query(ctx context.Context, action, statement string, args ...any) ([]entity.Memory, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}
	defer func() { _ = rows.Close() }()

	var memories []entity.Memory

	for rows.Next() {
		memory, err := scanMemory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}

		memories = append(memories, memory)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read memories: %w", err)
	}

	return memories, nil
}

func (r *memoryRepository) attachEvidence(ctx context.Context, memories []entity.Memory) error {
	if len(memories) == 0 {
		return nil
	}

	index := make(map[uuid.UUID]int, len(memories))
	ids := make([]string, 0, len(memories))

	for i, memory := range memories {
		index[memory.ID] = i
		ids = append(ids, memory.ID.String())
	}

	rows, err := r.db.Querier(ctx).QueryContext(ctx, evidenceQuery, ids)
	if err != nil {
		return fmt.Errorf("list memory evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		memoryID, evidence, err := scanEvidence(rows)
		if err != nil {
			return fmt.Errorf("scan memory evidence: %w", err)
		}

		at := index[memoryID]
		memories[at].Evidence = append(memories[at].Evidence, evidence)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("read memory evidence: %w", err)
	}

	return nil
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

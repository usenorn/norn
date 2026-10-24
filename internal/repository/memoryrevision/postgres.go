package memoryrevision

import (
	"context"
	"database/sql"
	"encoding/json"
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
       r.memory_id,
       r.version,
       r.action,
       r.status,
       r.statement,
       coalesce(r.account_id::text, ''),
       coalesce(a.display_name, ''),
       coalesce(r.agent_id::text, ''),
       coalesce(r.connection_id::text, ''),
       r.source,
       r.created_at`

const listRevisionsQuery = `
SELECT` + summaryColumns + `
FROM workspace_memory_revisions r
LEFT JOIN accounts a ON a.id = r.account_id
WHERE r.memory_id = $1 AND r.workspace_id = $2
  AND ($3::integer IS NULL OR r.version < $3::integer)
ORDER BY r.version DESC
LIMIT $4`

const revisionByIDQuery = `
SELECT` + summaryColumns + `,
       r.kind,
       r.rationale,
       r.conditions,
       r.visibility,
       coalesce(r.project_id::text, ''),
       coalesce(r.applies_project_id::text, ''),
       r.component,
       r.basis,
       r.effective_from,
       r.effective_until
FROM workspace_memory_revisions r
LEFT JOIN accounts a ON a.id = r.account_id
WHERE r.id = $1 AND r.memory_id = $2 AND r.workspace_id = $3`

const revisionEvidenceQuery = `
SELECT e.item ->> 'kind',
       coalesce(e.item ->> 'issue_id', ''),
       coalesce(e.item ->> 'comment_id', ''),
       coalesce(e.item ->> 'doc_id', ''),
       coalesce(e.item ->> 'doc_revision_id', ''),
       coalesce(e.item ->> 'excerpt', ''),
       coalesce(d.visibility, ''),
       coalesce(d.project_id::text, ''),
       coalesce(d.author_account_id::text, '')
FROM workspace_memory_revisions r
CROSS JOIN LATERAL jsonb_array_elements(r.evidence) WITH ORDINALITY AS e(item, position)
LEFT JOIN workspace_docs d ON d.id::text = e.item ->> 'doc_id'
WHERE r.id = $1
ORDER BY e.position`

type evidenceRecord struct {
	Kind          string     `json:"kind"`
	IssueID       *uuid.UUID `json:"issue_id,omitempty"`
	CommentID     *uuid.UUID `json:"comment_id,omitempty"`
	DocID         *uuid.UUID `json:"doc_id,omitempty"`
	DocRevisionID *uuid.UUID `json:"doc_revision_id,omitempty"`
	Excerpt       string     `json:"excerpt,omitempty"`
}

func revisionToModel(revision entity.MemoryRevision) (*dbpostgres.WorkspaceMemoryRevision, error) {
	records := make([]evidenceRecord, 0, len(revision.Evidence))
	for _, evidence := range revision.Evidence {
		records = append(records, evidenceRecord{
			Kind:          string(evidence.Kind),
			IssueID:       evidence.IssueID,
			CommentID:     evidence.CommentID,
			DocID:         evidence.DocID,
			DocRevisionID: evidence.DocRevisionID,
			Excerpt:       evidence.Excerpt,
		})
	}

	evidence, err := json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("encode memory revision evidence: %w", err)
	}

	projectID := null.String{}
	if revision.Audience.Visibility == entity.KnowledgeVisibilityProject {
		projectID = null.StringFrom(revision.Audience.ProjectID.String())
	}

	return &dbpostgres.WorkspaceMemoryRevision{
		ID:               revision.ID.String(),
		WorkspaceID:      revision.WorkspaceID.String(),
		MemoryID:         revision.MemoryID.String(),
		Version:          revision.Version,
		Action:           string(revision.Action),
		Kind:             string(revision.Kind),
		Statement:        revision.Statement,
		Rationale:        revision.Rationale,
		Conditions:       revision.Conditions,
		Visibility:       string(revision.Audience.Visibility),
		ProjectID:        projectID,
		AppliesProjectID: optionalUUID(revision.Applicability.ProjectID),
		Component:        revision.Applicability.Component,
		Status:           string(revision.Status),
		Basis:            string(revision.Basis),
		Evidence:         types.JSON(evidence),
		EffectiveFrom:    revision.EffectiveFrom,
		EffectiveUntil:   null.TimeFromPtr(revision.EffectiveUntil),
		AccountID:        null.NewString(revision.By.AccountID.String(), revision.By.AccountID != uuid.Nil),
		AgentID:          optionalUUID(revision.By.AgentID),
		ConnectionID:     optionalUUID(revision.By.ConnectionID),
		Source:           string(revision.By.Source),
		CreatedAt:        revision.CreatedAt,
	}, nil
}

func optionalUUID(id *uuid.UUID) null.String {
	if id == nil {
		return null.String{}
	}

	return null.StringFrom(id.String())
}

type summaryRow struct {
	id, workspace, memory, action, status, account, agent, connection, source string
}

func (row *summaryRow) targets(summary *entity.MemoryRevisionSummary) []any {
	return []any{
		&row.id, &row.workspace, &row.memory, &summary.Version, &row.action, &row.status, &summary.Statement,
		&row.account, &summary.AccountName, &row.agent, &row.connection, &row.source, &summary.CreatedAt,
	}
}

func (row *summaryRow) fill(summary *entity.MemoryRevisionSummary) error {
	var err error

	if summary.ID, err = uuid.Parse(row.id); err != nil {
		return fmt.Errorf("parse memory revision id: %w", err)
	}

	if summary.WorkspaceID, err = uuid.Parse(row.workspace); err != nil {
		return fmt.Errorf("parse memory revision workspace id: %w", err)
	}

	if summary.MemoryID, err = uuid.Parse(row.memory); err != nil {
		return fmt.Errorf("parse memory revision memory id: %w", err)
	}

	if summary.By.AccountID, err = parseOrNil(row.account); err != nil {
		return fmt.Errorf("parse memory revision account id: %w", err)
	}

	if summary.By.AgentID, err = parseOptional(row.agent); err != nil {
		return fmt.Errorf("parse memory revision agent id: %w", err)
	}

	if summary.By.ConnectionID, err = parseOptional(row.connection); err != nil {
		return fmt.Errorf("parse memory revision connection id: %w", err)
	}

	summary.Action = entity.MemoryRevisionAction(row.action)
	summary.Status = entity.MemoryStatus(row.status)
	summary.By.Source = entity.RevisionSource(row.source)
	summary.CreatedAt = summary.CreatedAt.UTC()

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

func New(db *postgres.Client) repository.MemoryRevision {
	return &revisionRepository{db: db}
}

func (r *revisionRepository) Record(ctx context.Context, revision entity.MemoryRevision) error {
	model, err := revisionToModel(revision)
	if err != nil {
		return err
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return fmt.Errorf("insert memory revision: %w", err)
	}

	return nil
}

func (r *revisionRepository) List(
	ctx context.Context,
	workspaceID, memoryID uuid.UUID,
	page entity.MemoryRevisionPage,
) ([]entity.MemoryRevisionSummary, error) {
	var before sql.NullInt64
	if page.Cursor != nil {
		before = sql.NullInt64{Int64: int64(page.Cursor.Version), Valid: true}
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listRevisionsQuery, memoryID.String(), workspaceID.String(), before, page.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list memory revisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []entity.MemoryRevisionSummary

	for rows.Next() {
		var (
			summary entity.MemoryRevisionSummary
			raw     summaryRow
		)

		if err := rows.Scan(raw.targets(&summary)...); err != nil {
			return nil, fmt.Errorf("scan memory revision: %w", err)
		}

		if err := raw.fill(&summary); err != nil {
			return nil, err
		}

		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read memory revisions: %w", err)
	}

	return summaries, nil
}

func (r *revisionRepository) GetByID(
	ctx context.Context,
	workspaceID, memoryID, revisionID uuid.UUID,
) (entity.MemoryRevision, error) {
	var (
		revision                                         entity.MemoryRevision
		raw                                              summaryRow
		kind, visibility, project, appliesProject, basis string
		effectiveUntil                                   sql.NullTime
	)

	row := r.db.Querier(ctx).QueryRowContext(
		ctx, revisionByIDQuery, revisionID.String(), memoryID.String(), workspaceID.String(),
	)

	if err := row.Scan(append(
		raw.targets(&revision.MemoryRevisionSummary),
		&kind, &revision.Rationale, &revision.Conditions, &visibility, &project, &appliesProject,
		&revision.Applicability.Component, &basis, &revision.EffectiveFrom, &effectiveUntil,
	)...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.MemoryRevision{}, entity.ErrMemoryRevisionNotFound
		}

		return entity.MemoryRevision{}, fmt.Errorf("find memory revision: %w", err)
	}

	if err := raw.fill(&revision.MemoryRevisionSummary); err != nil {
		return entity.MemoryRevision{}, err
	}

	projectID, err := parseOrNil(project)
	if err != nil {
		return entity.MemoryRevision{}, fmt.Errorf("parse memory revision project id: %w", err)
	}

	if revision.Applicability.ProjectID, err = parseOptional(appliesProject); err != nil {
		return entity.MemoryRevision{}, fmt.Errorf("parse memory revision applicability project id: %w", err)
	}

	revision.Kind = entity.MemoryKind(kind)
	revision.Audience = entity.KnowledgeAudience{Visibility: entity.KnowledgeVisibility(visibility), ProjectID: projectID}
	revision.Basis = entity.MemoryBasis(basis)
	revision.EffectiveFrom = entity.MemoryDay(revision.EffectiveFrom)

	if effectiveUntil.Valid {
		until := entity.MemoryDay(effectiveUntil.Time)
		revision.EffectiveUntil = &until
	}

	if revision.Evidence, err = r.evidence(ctx, revisionID); err != nil {
		return entity.MemoryRevision{}, err
	}

	return revision, nil
}

func (r *revisionRepository) evidence(ctx context.Context, revisionID uuid.UUID) ([]entity.MemoryEvidence, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, revisionEvidenceQuery, revisionID.String())
	if err != nil {
		return nil, fmt.Errorf("list memory revision evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cited []entity.MemoryEvidence

	for rows.Next() {
		var (
			evidence                               entity.MemoryEvidence
			kind, issue, comment, doc, docRevision string
			docVisibility, docProject, docAuthor   string
		)

		if err := rows.Scan(
			&kind, &issue, &comment, &doc, &docRevision, &evidence.Excerpt, &docVisibility, &docProject, &docAuthor,
		); err != nil {
			return nil, fmt.Errorf("scan memory revision evidence: %w", err)
		}

		evidence.Kind = entity.MemoryEvidenceKind(kind)

		for _, target := range []struct {
			raw string
			id  **uuid.UUID
		}{
			{issue, &evidence.IssueID},
			{comment, &evidence.CommentID},
			{doc, &evidence.DocID},
			{docRevision, &evidence.DocRevisionID},
		} {
			if *target.id, err = parseOptional(target.raw); err != nil {
				return nil, fmt.Errorf("parse memory revision evidence target id: %w", err)
			}
		}

		docProjectID, err := parseOrNil(docProject)
		if err != nil {
			return nil, fmt.Errorf("parse memory revision evidence doc project id: %w", err)
		}

		if evidence.DocAuthorID, err = parseOrNil(docAuthor); err != nil {
			return nil, fmt.Errorf("parse memory revision evidence doc author id: %w", err)
		}

		evidence.DocAudience = entity.KnowledgeAudience{
			Visibility: entity.KnowledgeVisibility(docVisibility),
			ProjectID:  docProjectID,
		}

		cited = append(cited, evidence)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read memory revision evidence: %w", err)
	}

	return cited, nil
}

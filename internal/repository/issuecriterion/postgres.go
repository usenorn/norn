package issuecriterion

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const evidenceColumns = `
       e.id,
       e.workspace_id,
       e.issue_id,
       e.criterion_id,
       e.criterion_text,
       e.kind,
       e.label,
       e.url,
       coalesce(e.attachment_id::text, ''),
       coalesce(e.description_revision_id::text, ''),
       coalesce(e.recorded_by_account_id::text, ''),
       coalesce(a.display_name, ''),
       e.recorded_at`

const recordEvidenceQuery = `
WITH recorded AS (
    INSERT INTO workspace_issue_criterion_evidence
        (id, workspace_id, issue_id, criterion_id, criterion_text, kind, label, url,
         attachment_id, description_revision_id, recorded_by_account_id, recorded_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, nullif($9, '')::uuid, nullif($10, '')::uuid,
            nullif($11, '')::uuid, $12)
    RETURNING *
)
SELECT` + evidenceColumns + `
FROM recorded e
LEFT JOIN accounts a ON a.id = e.recorded_by_account_id`

const listEvidenceQuery = `
SELECT` + evidenceColumns + `
FROM workspace_issue_criterion_evidence e
LEFT JOIN accounts a ON a.id = e.recorded_by_account_id
WHERE e.issue_id = $1 AND e.workspace_id = $2
ORDER BY e.recorded_at, e.id`

const countEvidenceQuery = `
SELECT count(*)
FROM workspace_issue_criterion_evidence
WHERE issue_id = $1 AND workspace_id = $2 AND criterion_id = $3`

const removeEvidenceQuery = `
DELETE FROM workspace_issue_criterion_evidence
WHERE id = $1 AND issue_id = $2 AND workspace_id = $3`

type criterionRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueCriterion {
	return &criterionRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEvidence(row scanner) (entity.CriterionEvidence, error) {
	var (
		evidence                   entity.CriterionEvidence
		id, workspace, issue       string
		kind, attachment, revision string
		recorder                   string
	)

	if err := row.Scan(
		&id, &workspace, &issue, &evidence.CriterionID, &evidence.CriterionText,
		&kind, &evidence.Label, &evidence.URL, &attachment, &revision,
		&recorder, &evidence.RecordedByName, &evidence.RecordedAt,
	); err != nil {
		return entity.CriterionEvidence{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.CriterionEvidence{}, fmt.Errorf("parse evidence id: %w", err)
	}

	evidence.ID = parsed

	if evidence.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.CriterionEvidence{}, fmt.Errorf("parse evidence workspace id: %w", err)
	}

	if evidence.IssueID, err = uuid.Parse(issue); err != nil {
		return entity.CriterionEvidence{}, fmt.Errorf("parse evidence issue id: %w", err)
	}

	for target, raw := range map[*uuid.UUID]string{
		&evidence.AttachmentID:          attachment,
		&evidence.DescriptionRevisionID: revision,
		&evidence.RecordedByAccountID:   recorder,
	} {
		if raw == "" {
			continue
		}

		if *target, err = uuid.Parse(raw); err != nil {
			return entity.CriterionEvidence{}, fmt.Errorf("parse evidence reference: %w", err)
		}
	}

	evidence.Kind = entity.EvidenceKind(kind)

	return evidence, nil
}

func (r *criterionRepository) Record(
	ctx context.Context,
	evidence entity.CriterionEvidence,
) (entity.CriterionEvidence, error) {
	if evidence.ID == uuid.Nil {
		evidence.ID = uuid.New()
	}

	recorded, err := scanEvidence(r.db.Querier(ctx).QueryRowContext(
		ctx, recordEvidenceQuery,
		evidence.ID.String(), evidence.WorkspaceID.String(), evidence.IssueID.String(),
		evidence.CriterionID, evidence.CriterionText, string(evidence.Kind),
		evidence.Label, evidence.URL, text(evidence.AttachmentID),
		text(evidence.DescriptionRevisionID), text(evidence.RecordedByAccountID),
		evidence.RecordedAt,
	))
	if err != nil {
		return entity.CriterionEvidence{}, fmt.Errorf("record evidence: %w", err)
	}

	return recorded, nil
}

func (r *criterionRepository) ListForIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.CriterionEvidence, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listEvidenceQuery, issueID.String(), workspaceID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var evidence []entity.CriterionEvidence

	for rows.Next() {
		held, err := scanEvidence(rows)
		if err != nil {
			return nil, fmt.Errorf("read evidence: %w", err)
		}

		evidence = append(evidence, held)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read evidence: %w", err)
	}

	return evidence, nil
}

func (r *criterionRepository) CountForCriterion(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	criterionID string,
) (int, error) {
	var counted int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, countEvidenceQuery, issueID.String(), workspaceID.String(), criterionID,
	).Scan(&counted); err != nil {
		return 0, fmt.Errorf("count evidence: %w", err)
	}

	return counted, nil
}

func (r *criterionRepository) Remove(
	ctx context.Context,
	workspaceID, issueID, evidenceID uuid.UUID,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, removeEvidenceQuery,
		evidenceID.String(), issueID.String(), workspaceID.String(),
	)
	if err != nil {
		return fmt.Errorf("remove evidence: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove evidence: %w", err)
	}

	if affected == 0 {
		return entity.ErrEvidenceNotFound
	}

	return nil
}

func text(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

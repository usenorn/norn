package check

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const evidenceColumns = `
    id, workspace_id, issue_id, check_id, verdict, channel, command, output,
    output_truncated, redactions, exit_code, observed_at, received_at,
    actor_kind, coalesce(actor_account_id::text, ''), actor_name,
    coalesce(code_link_id::text, ''), commit_sha,
    coalesce(scrubbed_by_account_id::text, ''), scrubbed_at`

const insertEvidenceQuery = `
INSERT INTO workspace_check_evidence (
    id, workspace_id, issue_id, check_id, verdict, channel, command, output,
    output_truncated, redactions, exit_code, observed_at, received_at,
    actor_kind, actor_account_id, actor_name, code_link_id, commit_sha
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
RETURNING` + evidenceColumns

const evidenceByCheckQuery = `
SELECT` + evidenceColumns + `
FROM workspace_check_evidence
WHERE workspace_id = $1 AND check_id = $2
ORDER BY received_at, id`

const evidenceByIssueQuery = `
SELECT` + evidenceColumns + `
FROM workspace_check_evidence
WHERE workspace_id = $1 AND issue_id = $2
ORDER BY received_at, id`

type evidenceRepository struct {
	db *postgres.Client
}

func NewEvidence(db *postgres.Client) repository.CheckEvidence {
	return &evidenceRepository{db: db}
}

func scanEvidence(row scanner) (entity.Evidence, error) {
	var (
		evidence    entity.Evidence
		id          string
		workspaceID string
		issueID     string
		checkID     string
		verdict     string
		channel     string
		actorKind   string
		actorID     string
		codeLinkID  string
		scrubbedBy  string
		exitCode    sql.NullInt64
		scrubbedAt  sql.NullTime
	)

	if err := row.Scan(
		&id,
		&workspaceID,
		&issueID,
		&checkID,
		&verdict,
		&channel,
		&evidence.Command,
		&evidence.Output,
		&evidence.Truncated,
		&evidence.Redactions,
		&exitCode,
		&evidence.ObservedAt,
		&evidence.ReceivedAt,
		&actorKind,
		&actorID,
		&evidence.ActorName,
		&codeLinkID,
		&evidence.CommitSHA,
		&scrubbedBy,
		&scrubbedAt,
	); err != nil {
		return entity.Evidence{}, err
	}

	evidence.Verdict = entity.EvidenceVerdict(verdict)
	evidence.Channel = entity.EvidenceChannel(channel)
	evidence.Actor.Kind = entity.ActorKind(actorKind)

	if exitCode.Valid {
		code := int(exitCode.Int64)
		evidence.ExitCode = &code
	}

	if scrubbedAt.Valid {
		evidence.ScrubbedAt = &scrubbedAt.Time
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.Evidence{}, fmt.Errorf("parse evidence id: %w", err)
	}

	evidence.ID = parsed

	if evidence.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return entity.Evidence{}, fmt.Errorf("parse evidence workspace id: %w", err)
	}

	if evidence.IssueID, err = uuid.Parse(issueID); err != nil {
		return entity.Evidence{}, fmt.Errorf("parse evidence issue id: %w", err)
	}

	if evidence.CheckID, err = uuid.Parse(checkID); err != nil {
		return entity.Evidence{}, fmt.Errorf("parse evidence check id: %w", err)
	}

	if evidence.Actor.AccountID, err = optionalID(actorID); err != nil {
		return entity.Evidence{}, fmt.Errorf("parse evidence actor id: %w", err)
	}

	if evidence.CodeLinkID, err = optionalID(codeLinkID); err != nil {
		return entity.Evidence{}, fmt.Errorf("parse evidence code link id: %w", err)
	}

	if evidence.ScrubbedByAccountID, err = optionalID(scrubbedBy); err != nil {
		return entity.Evidence{}, fmt.Errorf("parse evidence scrubber id: %w", err)
	}

	return evidence, nil
}

func (r *evidenceRepository) Append(
	ctx context.Context,
	evidence entity.Evidence,
) (entity.Evidence, error) {
	if evidence.ID == uuid.Nil {
		evidence.ID = uuid.New()
	}

	if evidence.ReceivedAt.IsZero() {
		evidence.ReceivedAt = time.Now().UTC()
	}

	var exitCode any

	if evidence.ExitCode != nil {
		exitCode = *evidence.ExitCode
	}

	stored, err := scanEvidence(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertEvidenceQuery,
		evidence.ID.String(),
		evidence.WorkspaceID.String(),
		evidence.IssueID.String(),
		evidence.CheckID.String(),
		string(evidence.Verdict),
		string(evidence.Channel),
		evidence.Command,
		evidence.Output,
		evidence.Truncated,
		evidence.Redactions,
		exitCode,
		evidence.ObservedAt,
		evidence.ReceivedAt,
		string(evidence.Actor.Kind),
		idOrNil(evidence.Actor.AccountID),
		evidence.ActorName,
		idOrNil(evidence.CodeLinkID),
		evidence.CommitSHA,
	))
	if err != nil {
		return entity.Evidence{}, fmt.Errorf("insert evidence: %w", err)
	}

	return stored, nil
}

func (r *evidenceRepository) ListByCheck(
	ctx context.Context,
	workspaceID, checkID uuid.UUID,
) ([]entity.Evidence, error) {
	return r.list(ctx, evidenceByCheckQuery, workspaceID, checkID)
}

func (r *evidenceRepository) ListByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Evidence, error) {
	return r.list(ctx, evidenceByIssueQuery, workspaceID, issueID)
}

func (r *evidenceRepository) list(
	ctx context.Context,
	query string,
	workspaceID, subjectID uuid.UUID,
) ([]entity.Evidence, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		query,
		workspaceID.String(),
		subjectID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}

	defer func() { _ = rows.Close() }()

	records := make([]entity.Evidence, 0)

	for rows.Next() {
		record, err := scanEvidence(rows)
		if err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate evidence: %w", err)
	}

	return records, nil
}

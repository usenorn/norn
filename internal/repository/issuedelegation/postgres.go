package issuedelegation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolation = "23505"
	openUniqueIndex = "workspace_issue_delegations_open_key"
)

const delegationColumns = `
	d.id, d.workspace_id, d.issue_id, d.agent_id, a.name, a.account_id, d.brief,
	coalesce(d.delegated_by_account_id::text, ''), d.delegated_at,
	coalesce(d.recalled_by_account_id::text, ''), d.recalled_at`

const delegationJoins = `
FROM workspace_issue_delegations d
JOIN workspace_agents a ON a.id = d.agent_id`

const insertDelegationQuery = `
INSERT INTO workspace_issue_delegations (
    id, workspace_id, issue_id, agent_id, brief, delegated_by_account_id, delegated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

const openDelegationQuery = `
SELECT` + delegationColumns + delegationJoins + `
WHERE d.workspace_id = $1 AND d.issue_id = $2 AND d.recalled_at IS NULL`

const delegationsByIssueQuery = `
SELECT` + delegationColumns + delegationJoins + `
WHERE d.workspace_id = $1 AND d.issue_id = $2
ORDER BY d.delegated_at DESC, d.id DESC`

const delegationByIDQuery = `
SELECT` + delegationColumns + delegationJoins + `
WHERE d.workspace_id = $1 AND d.id = $2`

const recallDelegationQuery = `
UPDATE workspace_issue_delegations
SET recalled_at = $3, recalled_by_account_id = $4
WHERE workspace_id = $1 AND issue_id = $2 AND recalled_at IS NULL
RETURNING id`

type delegationRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueDelegation {
	return &delegationRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDelegation(row scanner) (entity.IssueDelegation, error) {
	var (
		delegation   entity.IssueDelegation
		id           string
		workspaceID  string
		issueID      string
		agentID      string
		agentAccount string
		delegatedBy  string
		recalledBy   string
		recalledAt   sql.NullTime
	)

	if err := row.Scan(
		&id,
		&workspaceID,
		&issueID,
		&agentID,
		&delegation.AgentName,
		&agentAccount,
		&delegation.Brief,
		&delegatedBy,
		&delegation.DelegatedAt,
		&recalledBy,
		&recalledAt,
	); err != nil {
		return entity.IssueDelegation{}, err
	}

	if recalledAt.Valid {
		delegation.RecalledAt = &recalledAt.Time
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.IssueDelegation{}, fmt.Errorf("parse delegation id: %w", err)
	}

	delegation.ID = parsed

	if delegation.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return entity.IssueDelegation{}, fmt.Errorf("parse delegation workspace id: %w", err)
	}

	if delegation.IssueID, err = uuid.Parse(issueID); err != nil {
		return entity.IssueDelegation{}, fmt.Errorf("parse delegation issue id: %w", err)
	}

	if delegation.AgentID, err = uuid.Parse(agentID); err != nil {
		return entity.IssueDelegation{}, fmt.Errorf("parse delegation agent id: %w", err)
	}

	if delegation.AgentAccountID, err = uuid.Parse(agentAccount); err != nil {
		return entity.IssueDelegation{}, fmt.Errorf("parse delegation agent account id: %w", err)
	}

	if delegatedBy != "" {
		if delegation.DelegatedByAccountID, err = uuid.Parse(delegatedBy); err != nil {
			return entity.IssueDelegation{}, fmt.Errorf("parse delegation author id: %w", err)
		}
	}

	if recalledBy != "" {
		if delegation.RecalledByAccountID, err = uuid.Parse(recalledBy); err != nil {
			return entity.IssueDelegation{}, fmt.Errorf("parse delegation recaller id: %w", err)
		}
	}

	return delegation, nil
}

func (r *delegationRepository) Open(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.IssueDelegation, error) {
	delegation, err := scanDelegation(r.db.Querier(ctx).QueryRowContext(
		ctx,
		openDelegationQuery,
		workspaceID.String(),
		issueID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IssueDelegation{}, entity.ErrIssueDelegationNotFound
		}

		return entity.IssueDelegation{}, fmt.Errorf("find open issue delegation: %w", err)
	}

	return delegation, nil
}

func (r *delegationRepository) ListByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.IssueDelegation, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		delegationsByIssueQuery,
		workspaceID.String(),
		issueID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list issue delegations: %w", err)
	}

	defer func() { _ = rows.Close() }()

	delegations := make([]entity.IssueDelegation, 0)

	for rows.Next() {
		delegation, err := scanDelegation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issue delegation: %w", err)
		}

		delegations = append(delegations, delegation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue delegations: %w", err)
	}

	return delegations, nil
}

func (r *delegationRepository) Delegate(
	ctx context.Context,
	delegation entity.IssueDelegation,
) (entity.IssueDelegation, error) {
	if delegation.ID == uuid.Nil {
		delegation.ID = uuid.New()
	}

	if delegation.DelegatedAt.IsZero() {
		delegation.DelegatedAt = time.Now().UTC()
	}

	var author any

	if delegation.DelegatedByAccountID != uuid.Nil {
		author = delegation.DelegatedByAccountID.String()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		insertDelegationQuery,
		delegation.ID.String(),
		delegation.WorkspaceID.String(),
		delegation.IssueID.String(),
		delegation.AgentID.String(),
		delegation.Brief,
		author,
		delegation.DelegatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == uniqueViolation &&
			pgErr.ConstraintName == openUniqueIndex {
			return entity.IssueDelegation{}, entity.ErrIssueDelegationHeld
		}

		return entity.IssueDelegation{}, fmt.Errorf("insert issue delegation: %w", err)
	}

	return r.byID(ctx, delegation.WorkspaceID, delegation.ID)
}

func (r *delegationRepository) Recall(
	ctx context.Context,
	workspaceID uuid.UUID,
	recall repository.RecallDelegation,
) (entity.IssueDelegation, error) {
	var recaller any

	if recall.AccountID != uuid.Nil {
		recaller = recall.AccountID.String()
	}

	var id string

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		recallDelegationQuery,
		workspaceID.String(),
		recall.IssueID.String(),
		recall.RecalledAt,
		recaller,
	).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IssueDelegation{}, entity.ErrIssueDelegationNotFound
		}

		return entity.IssueDelegation{}, fmt.Errorf("recall issue delegation: %w", err)
	}

	recalled, err := uuid.Parse(id)
	if err != nil {
		return entity.IssueDelegation{}, fmt.Errorf("parse recalled delegation id: %w", err)
	}

	return r.byID(ctx, workspaceID, recalled)
}

func (r *delegationRepository) byID(
	ctx context.Context,
	workspaceID, delegationID uuid.UUID,
) (entity.IssueDelegation, error) {
	delegation, err := scanDelegation(r.db.Querier(ctx).QueryRowContext(
		ctx,
		delegationByIDQuery,
		workspaceID.String(),
		delegationID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IssueDelegation{}, entity.ErrIssueDelegationNotFound
		}

		return entity.IssueDelegation{}, fmt.Errorf("find issue delegation: %w", err)
	}

	return delegation, nil
}

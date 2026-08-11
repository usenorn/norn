package check

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const checkColumns = `
    id, workspace_id, issue_id, position, statement, method, proof, time_limit_seconds,
    approval, coalesce(approved_by_account_id::text, ''), approved_at,
    resolution, resolution_reason, coalesce(resolved_by_account_id::text, ''), resolved_at,
    coalesce(gap_issue_id::text, ''), author_kind, coalesce(created_by_account_id::text, ''),
    added_after_delegation, created_at, updated_at`

const insertCheckQuery = `
INSERT INTO workspace_issue_checks (
    id, workspace_id, issue_id, position, statement, method, proof, time_limit_seconds,
    approval, approved_by_account_id, approved_at, author_kind, created_by_account_id,
    added_after_delegation
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING` + checkColumns

const checkByIDQuery = `
SELECT` + checkColumns + `
FROM workspace_issue_checks
WHERE workspace_id = $1 AND id = $2`

const checksByIssueQuery = `
SELECT` + checkColumns + `
FROM workspace_issue_checks
WHERE workspace_id = $1 AND issue_id = $2
ORDER BY position, id`

const decideCheckQuery = `
UPDATE workspace_issue_checks
SET approval = $3,
    approved_by_account_id = $4,
    approved_at = $5,
    updated_at = now()
WHERE workspace_id = $1 AND id = $2
RETURNING` + checkColumns

const resolveCheckQuery = `
UPDATE workspace_issue_checks
SET resolution = $3,
    resolution_reason = $4,
    gap_issue_id = $5,
    resolved_by_account_id = $6,
    resolved_at = $7,
    updated_at = now()
WHERE workspace_id = $1 AND id = $2
RETURNING` + checkColumns

const deleteCheckQuery = `
DELETE FROM workspace_issue_checks WHERE workspace_id = $1 AND issue_id = $2 AND id = $3`

type checkRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Check {
	return &checkRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCheck(row scanner) (entity.Check, error) {
	var (
		check       entity.Check
		id          string
		workspaceID string
		issueID     string
		method      string
		approval    string
		approvedBy  string
		approvedAt  sql.NullTime
		resolution  string
		resolvedBy  string
		resolvedAt  sql.NullTime
		gapIssue    string
		authorKind  string
		createdBy   string
		timeLimit   sql.NullInt64
	)

	if err := row.Scan(
		&id,
		&workspaceID,
		&issueID,
		&check.Position,
		&check.Statement,
		&method,
		&check.Proof,
		&timeLimit,
		&approval,
		&approvedBy,
		&approvedAt,
		&resolution,
		&check.ResolutionReason,
		&resolvedBy,
		&resolvedAt,
		&gapIssue,
		&authorKind,
		&createdBy,
		&check.AddedAfterDelegation,
		&check.CreatedAt,
		&check.UpdatedAt,
	); err != nil {
		return entity.Check{}, err
	}

	check.Method = entity.CheckMethod(method)
	check.Approval = entity.CheckApproval(approval)
	check.Resolution = entity.CheckResolution(resolution)
	check.AuthorKind = entity.ActorKind(authorKind)

	if timeLimit.Valid {
		window := time.Duration(timeLimit.Int64) * time.Second
		check.TimeLimit = &window
	}

	if approvedAt.Valid {
		check.ApprovedAt = &approvedAt.Time
	}

	if resolvedAt.Valid {
		check.ResolvedAt = &resolvedAt.Time
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.Check{}, fmt.Errorf("parse check id: %w", err)
	}

	check.ID = parsed

	if check.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return entity.Check{}, fmt.Errorf("parse check workspace id: %w", err)
	}

	if check.IssueID, err = uuid.Parse(issueID); err != nil {
		return entity.Check{}, fmt.Errorf("parse check issue id: %w", err)
	}

	if check.ApprovedByAccountID, err = optionalID(approvedBy); err != nil {
		return entity.Check{}, fmt.Errorf("parse check approver id: %w", err)
	}

	if check.ResolvedByAccountID, err = optionalID(resolvedBy); err != nil {
		return entity.Check{}, fmt.Errorf("parse check resolver id: %w", err)
	}

	if check.GapIssueID, err = optionalID(gapIssue); err != nil {
		return entity.Check{}, fmt.Errorf("parse check gap issue id: %w", err)
	}

	if check.CreatedByAccountID, err = optionalID(createdBy); err != nil {
		return entity.Check{}, fmt.Errorf("parse check author id: %w", err)
	}

	return check, nil
}

func optionalID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(value)
}

func idOrNil(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}

	return id.String()
}

func (r *checkRepository) Create(ctx context.Context, check entity.Check) (entity.Check, error) {
	if check.ID == uuid.Nil {
		check.ID = uuid.New()
	}

	var (
		window     any
		approvedAt any
	)

	if check.TimeLimit != nil {
		window = int64(*check.TimeLimit / time.Second)
	}

	if check.ApprovedAt != nil {
		approvedAt = *check.ApprovedAt
	}

	created, err := scanCheck(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertCheckQuery,
		check.ID.String(),
		check.WorkspaceID.String(),
		check.IssueID.String(),
		check.Position,
		check.Statement,
		string(check.Method),
		check.Proof,
		window,
		string(check.Approval),
		idOrNil(check.ApprovedByAccountID),
		approvedAt,
		string(check.AuthorKind),
		idOrNil(check.CreatedByAccountID),
		check.AddedAfterDelegation,
	))
	if err != nil {
		return entity.Check{}, fmt.Errorf("insert check: %w", err)
	}

	return created, nil
}

func (r *checkRepository) GetByID(
	ctx context.Context,
	workspaceID, checkID uuid.UUID,
) (entity.Check, error) {
	check, err := scanCheck(r.db.Querier(ctx).QueryRowContext(
		ctx,
		checkByIDQuery,
		workspaceID.String(),
		checkID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Check{}, entity.ErrCheckNotFound
		}

		return entity.Check{}, fmt.Errorf("find check: %w", err)
	}

	return check, nil
}

func (r *checkRepository) ListByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Check, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		checksByIssueQuery,
		workspaceID.String(),
		issueID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list checks: %w", err)
	}

	defer func() { _ = rows.Close() }()

	checks := make([]entity.Check, 0)

	for rows.Next() {
		check, err := scanCheck(rows)
		if err != nil {
			return nil, fmt.Errorf("scan check: %w", err)
		}

		checks = append(checks, check)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate checks: %w", err)
	}

	return checks, nil
}

func (r *checkRepository) Decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision repository.CheckDecision,
) (entity.Check, error) {
	var decidedAt any

	if decision.Approval.Decided() {
		decidedAt = decision.DecidedAt
	}

	decided, err := scanCheck(r.db.Querier(ctx).QueryRowContext(
		ctx,
		decideCheckQuery,
		workspaceID.String(),
		decision.CheckID.String(),
		string(decision.Approval),
		idOrNil(decision.AccountID),
		decidedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Check{}, entity.ErrCheckNotFound
		}

		return entity.Check{}, fmt.Errorf("decide check: %w", err)
	}

	return decided, nil
}

func (r *checkRepository) Resolve(
	ctx context.Context,
	workspaceID uuid.UUID,
	resolution repository.CheckResolutionInput,
) (entity.Check, error) {
	var resolvedAt any

	if resolution.Resolution != entity.CheckResolutionNone {
		resolvedAt = resolution.ResolvedAt
	}

	resolved, err := scanCheck(r.db.Querier(ctx).QueryRowContext(
		ctx,
		resolveCheckQuery,
		workspaceID.String(),
		resolution.CheckID.String(),
		string(resolution.Resolution),
		resolution.Reason,
		idOrNil(resolution.GapIssueID),
		idOrNil(resolution.AccountID),
		resolvedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Check{}, entity.ErrCheckNotFound
		}

		return entity.Check{}, fmt.Errorf("resolve check: %w", err)
	}

	return resolved, nil
}

func (r *checkRepository) Delete(ctx context.Context, workspaceID, issueID, checkID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		deleteCheckQuery,
		workspaceID.String(),
		issueID.String(),
		checkID.String(),
	)
	if err != nil {
		return fmt.Errorf("delete check: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read removed check rows: %w", err)
	}

	if removed == 0 {
		return entity.ErrCheckNotFound
	}

	return nil
}

package executionplan

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

const planColumns = `
    p.id, p.execution_id, p.workspace_id, p.revision, p.ref, p.body, p.proposed_at,
    coalesce(p.approved_by_account_id::text, ''), coalesce(approver.display_name, ''),
    p.approved_at, p.revision_feedback,
    coalesce(p.revision_requested_by::text, ''), coalesce(requester.display_name, ''),
    p.revision_requested_at`

const planNames = `
LEFT JOIN accounts approver ON approver.id = p.approved_by_account_id
LEFT JOIN accounts requester ON requester.id = p.revision_requested_by`

const proposePlanQuery = `
WITH proposed AS (
    INSERT INTO workspace_execution_plans
        (execution_id, workspace_id, revision, ref, body, proposed_at)
    SELECT $1, $2,
           coalesce((SELECT max(revision) FROM workspace_execution_plans WHERE execution_id = $1), 0) + 1,
           $3, $4, $5
    ON CONFLICT (execution_id, ref) DO NOTHING
    RETURNING *
)
SELECT` + planColumns + `
FROM proposed p` + planNames

const plansByExecutionQuery = `
SELECT` + planColumns + `
FROM workspace_execution_plans p` + planNames + `
WHERE p.execution_id = $1
ORDER BY p.revision`

const approvePlanQuery = `
WITH approved AS (
    UPDATE workspace_execution_plans
    SET approved_by_account_id = nullif($3, '')::uuid, approved_at = $4
    WHERE execution_id = $1 AND revision = $2
      AND approved_at IS NULL AND revision_requested_at IS NULL
      AND revision = (SELECT max(revision) FROM workspace_execution_plans WHERE execution_id = $1)
    RETURNING *
)
SELECT` + planColumns + `
FROM approved p` + planNames

const requestPlanRevisionQuery = `
WITH requested AS (
    UPDATE workspace_execution_plans
    SET revision_requested_by = nullif($3, '')::uuid, revision_requested_at = $4,
        revision_feedback = $5
    WHERE execution_id = $1 AND revision = $2
      AND approved_at IS NULL AND revision_requested_at IS NULL
      AND revision = (SELECT max(revision) FROM workspace_execution_plans WHERE execution_id = $1)
    RETURNING *
)
SELECT` + planColumns + `
FROM requested p` + planNames

type planRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.ExecutionPlan {
	return &planRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPlan(row scanner) (entity.ExecutionPlan, error) {
	var (
		plan        entity.ExecutionPlan
		id          string
		workspaceID string
		approvedBy  string
		requestedBy string
		approvedAt  sql.NullTime
		requestedAt sql.NullTime
	)

	if err := row.Scan(
		&id,
		&plan.ExecutionID,
		&workspaceID,
		&plan.Revision,
		&plan.Ref,
		&plan.Body,
		&plan.ProposedAt,
		&approvedBy,
		&plan.ApprovedByName,
		&approvedAt,
		&plan.RevisionFeedback,
		&requestedBy,
		&plan.RevisionRequestedByName,
		&requestedAt,
	); err != nil {
		return entity.ExecutionPlan{}, err
	}

	var err error

	if plan.ID, err = uuid.Parse(id); err != nil {
		return entity.ExecutionPlan{}, fmt.Errorf("parse plan id: %w", err)
	}

	if plan.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return entity.ExecutionPlan{}, fmt.Errorf("parse plan workspace id: %w", err)
	}

	if plan.ApprovedByAccountID, err = optionalID(approvedBy); err != nil {
		return entity.ExecutionPlan{}, fmt.Errorf("parse plan approver id: %w", err)
	}

	if plan.RevisionRequestedByID, err = optionalID(requestedBy); err != nil {
		return entity.ExecutionPlan{}, fmt.Errorf("parse plan revision requester id: %w", err)
	}

	if approvedAt.Valid {
		plan.ApprovedAt = &approvedAt.Time
	}

	if requestedAt.Valid {
		plan.RevisionRequestedAt = &requestedAt.Time
	}

	return plan, nil
}

func optionalID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(value)
}

func idOrEmpty(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func (r *planRepository) Propose(
	ctx context.Context,
	plan entity.ExecutionPlan,
) (entity.ExecutionPlan, error) {
	proposed, err := scanPlan(r.db.Querier(ctx).QueryRowContext(
		ctx,
		proposePlanQuery,
		plan.ExecutionID,
		plan.WorkspaceID.String(),
		plan.Ref,
		plan.Body,
		plan.ProposedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionPlan{}, entity.ErrExecutionPlanRecorded
		}

		return entity.ExecutionPlan{}, fmt.Errorf("propose plan: %w", err)
	}

	return proposed, nil
}

func (r *planRepository) ListByExecution(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionPlan, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, plansByExecutionQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}

	defer func() { _ = rows.Close() }()

	plans := make([]entity.ExecutionPlan, 0)

	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}

		plans = append(plans, plan)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plans: %w", err)
	}

	return plans, nil
}

func (r *planRepository) Approve(
	ctx context.Context,
	decision repository.PlanDecision,
) (entity.ExecutionPlan, error) {
	return r.decide(ctx, "approve plan", approvePlanQuery,
		decision.ExecutionID, decision.Revision, idOrEmpty(decision.AccountID), decision.At,
	)
}

func (r *planRepository) RequestRevision(
	ctx context.Context,
	decision repository.PlanDecision,
) (entity.ExecutionPlan, error) {
	return r.decide(ctx, "request plan revision", requestPlanRevisionQuery,
		decision.ExecutionID, decision.Revision, idOrEmpty(decision.AccountID), decision.At,
		decision.Feedback,
	)
}

func (r *planRepository) decide(
	ctx context.Context,
	action string,
	query string,
	args ...any,
) (entity.ExecutionPlan, error) {
	decided, err := scanPlan(r.db.Querier(ctx).QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionPlan{}, entity.ErrExecutionPlanStale
		}

		return entity.ExecutionPlan{}, fmt.Errorf("%s: %w", action, err)
	}

	return decided, nil
}

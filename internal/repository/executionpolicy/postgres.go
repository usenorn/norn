package executionpolicy

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

const policyColumns = `
       workspace_id,
       telemetry,
       upload_retention_days,
       created_at,
       updated_at`

const executionPolicyQuery = `
SELECT` + policyColumns + `
FROM workspace_execution_policies
WHERE workspace_id = $1`

const upsertExecutionPolicyQuery = `
INSERT INTO workspace_execution_policies (workspace_id, telemetry, upload_retention_days)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id) DO UPDATE SET
    telemetry = excluded.telemetry,
    upload_retention_days = excluded.upload_retention_days,
    updated_at = now()
RETURNING` + policyColumns

type executionPolicyRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.ExecutionPolicy {
	return &executionPolicyRepository{db: db}
}

func scanPolicy(row *sql.Row) (entity.WorkspaceExecutionPolicy, error) {
	var (
		policy      entity.WorkspaceExecutionPolicy
		workspaceID string
		telemetry   string
	)

	if err := row.Scan(
		&workspaceID,
		&telemetry,
		&policy.UploadRetentionDays,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	); err != nil {
		return entity.WorkspaceExecutionPolicy{}, err
	}

	parsed, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.WorkspaceExecutionPolicy{}, fmt.Errorf("parse workspace id: %w", err)
	}

	policy.WorkspaceID = parsed
	policy.Telemetry = entity.TelemetryMode(telemetry)

	return policy, nil
}

func (r *executionPolicyRepository) Policy(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.WorkspaceExecutionPolicy, error) {
	policy, err := scanPolicy(r.db.Querier(ctx).QueryRowContext(
		ctx, executionPolicyQuery, workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.WorkspaceExecutionPolicy{WorkspaceID: workspaceID}, nil
		}

		return entity.WorkspaceExecutionPolicy{}, fmt.Errorf("read execution policy: %w", err)
	}

	return policy, nil
}

func (r *executionPolicyRepository) Upsert(
	ctx context.Context,
	policy entity.WorkspaceExecutionPolicy,
) (entity.WorkspaceExecutionPolicy, error) {
	saved, err := scanPolicy(r.db.Querier(ctx).QueryRowContext(
		ctx,
		upsertExecutionPolicyQuery,
		policy.WorkspaceID.String(),
		string(policy.Telemetry),
		policy.UploadRetentionDays,
	))
	if err != nil {
		return entity.WorkspaceExecutionPolicy{}, fmt.Errorf("save execution policy: %w", err)
	}

	return saved, nil
}

package executionpolicy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type executionPolicyRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.ExecutionPolicy {
	return &executionPolicyRepository{db: db}
}

func toEntity(
	model *dbpostgres.WorkspaceExecutionPolicy,
) (entity.WorkspaceExecutionPolicy, error) {
	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.WorkspaceExecutionPolicy{}, fmt.Errorf("parse workspace id: %w", err)
	}

	return entity.WorkspaceExecutionPolicy{
		WorkspaceID:         workspaceID,
		Telemetry:           entity.TelemetryMode(model.Telemetry),
		UploadRetentionDays: model.UploadRetentionDays,
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
	}, nil
}

func toModel(policy entity.WorkspaceExecutionPolicy) *dbpostgres.WorkspaceExecutionPolicy {
	return &dbpostgres.WorkspaceExecutionPolicy{
		WorkspaceID:         policy.WorkspaceID.String(),
		Telemetry:           string(policy.Telemetry),
		UploadRetentionDays: policy.UploadRetentionDays,
		CreatedAt:           policy.CreatedAt,
		UpdatedAt:           policy.UpdatedAt,
	}
}

func (r *executionPolicyRepository) Policy(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.WorkspaceExecutionPolicy, error) {
	model, err := dbpostgres.FindWorkspaceExecutionPolicy(
		ctx, r.db.Querier(ctx), workspaceID.String(),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.WorkspaceExecutionPolicy{WorkspaceID: workspaceID}, nil
		}

		return entity.WorkspaceExecutionPolicy{}, fmt.Errorf("read execution policy: %w", err)
	}

	return toEntity(model)
}

func (r *executionPolicyRepository) Upsert(
	ctx context.Context,
	policy entity.WorkspaceExecutionPolicy,
) (entity.WorkspaceExecutionPolicy, error) {
	now := time.Now().UTC()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	model := toModel(policy)

	if err := model.Upsert(
		ctx,
		r.db.Querier(ctx),
		true,
		[]string{dbpostgres.WorkspaceExecutionPolicyColumns.WorkspaceID},
		boil.Whitelist(
			dbpostgres.WorkspaceExecutionPolicyColumns.Telemetry,
			dbpostgres.WorkspaceExecutionPolicyColumns.UploadRetentionDays,
			dbpostgres.WorkspaceExecutionPolicyColumns.UpdatedAt,
		),
		boil.Infer(),
	); err != nil {
		return entity.WorkspaceExecutionPolicy{}, fmt.Errorf("save execution policy: %w", err)
	}

	return toEntity(model)
}

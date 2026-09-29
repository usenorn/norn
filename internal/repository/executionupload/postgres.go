package executionupload

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	artifactUniqueIndex = "workspace_execution_artifacts_digest_key"
)

const uploadedBytesQuery = `
SELECT coalesce(sum(bytes), 0) FROM workspace_execution_artifacts WHERE execution_id = $1`

func artifactOf(model *dbpostgres.WorkspaceExecutionArtifact) (entity.ExecutionArtifact, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.ExecutionArtifact{}, fmt.Errorf("parse artifact id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.ExecutionArtifact{}, fmt.Errorf("parse workspace id: %w", err)
	}

	return entity.ExecutionArtifact{
		ID:          id,
		ExecutionID: model.ExecutionID,
		WorkspaceID: workspaceID,
		Name:        model.Name,
		ContentType: model.ContentType,
		Bytes:       model.Bytes,
		Digest:      model.Digest,
		ObjectKey:   model.ObjectKey,
		CreatedAt:   model.CreatedAt,
	}, nil
}

func artifactsOf(
	models dbpostgres.WorkspaceExecutionArtifactSlice,
) ([]entity.ExecutionArtifact, error) {
	artifacts := make([]entity.ExecutionArtifact, 0, len(models))

	for _, model := range models {
		artifact, err := artifactOf(model)
		if err != nil {
			return nil, err
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

type executionUploadRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.ExecutionUpload {
	return &executionUploadRepository{db: db}
}

func (r *executionUploadRepository) UploadedBytes(
	ctx context.Context,
	executionID string,
) (int64, error) {
	var stored int64

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, uploadedBytesQuery, executionID,
	).Scan(&stored); err != nil {
		return 0, fmt.Errorf("read what an execution has uploaded: %w", err)
	}

	return stored, nil
}

func (r *executionUploadRepository) SaveArtifact(
	ctx context.Context,
	artifact entity.ExecutionArtifact,
) (entity.ExecutionArtifact, error) {
	model := &dbpostgres.WorkspaceExecutionArtifact{
		ID:          artifact.ID.String(),
		ExecutionID: artifact.ExecutionID,
		WorkspaceID: artifact.WorkspaceID.String(),
		Name:        artifact.Name,
		ContentType: artifact.ContentType,
		Bytes:       artifact.Bytes,
		Digest:      artifact.Digest,
		ObjectKey:   artifact.ObjectKey,
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == uniqueViolationCode &&
			pgErr.ConstraintName == artifactUniqueIndex {
			return entity.ExecutionArtifact{}, entity.ErrExecutionUploadRecorded
		}

		return entity.ExecutionArtifact{}, fmt.Errorf("save execution artifact: %w", err)
	}

	return artifactOf(model)
}

func (r *executionUploadRepository) Artifact(
	ctx context.Context,
	executionID string,
	artifactID uuid.UUID,
) (entity.ExecutionArtifact, error) {
	model, err := dbpostgres.WorkspaceExecutionArtifacts(
		dbpostgres.WorkspaceExecutionArtifactWhere.ExecutionID.EQ(executionID),
		dbpostgres.WorkspaceExecutionArtifactWhere.ID.EQ(artifactID.String()),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionArtifact{}, entity.ErrExecutionArtifactNotFound
		}

		return entity.ExecutionArtifact{}, fmt.Errorf("read execution artifact: %w", err)
	}

	return artifactOf(model)
}

func (r *executionUploadRepository) ArtifactByDigest(
	ctx context.Context,
	executionID, digest string,
) (entity.ExecutionArtifact, error) {
	model, err := dbpostgres.WorkspaceExecutionArtifacts(
		dbpostgres.WorkspaceExecutionArtifactWhere.ExecutionID.EQ(executionID),
		dbpostgres.WorkspaceExecutionArtifactWhere.Digest.EQ(digest),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionArtifact{}, entity.ErrExecutionArtifactNotFound
		}

		return entity.ExecutionArtifact{}, fmt.Errorf("read execution artifact: %w", err)
	}

	return artifactOf(model)
}

func (r *executionUploadRepository) ListArtifacts(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionArtifact, error) {
	models, err := dbpostgres.WorkspaceExecutionArtifacts(
		dbpostgres.WorkspaceExecutionArtifactWhere.ExecutionID.EQ(executionID),
		qm.OrderBy(
			dbpostgres.WorkspaceExecutionArtifactColumns.CreatedAt+", "+
				dbpostgres.WorkspaceExecutionArtifactColumns.ID,
		),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list execution artifacts: %w", err)
	}

	return artifactsOf(models)
}

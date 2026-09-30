package executionsnapshot

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const claimRevisionQuery = `
INSERT INTO workspace_execution_snapshots
    (execution_id, workspace_id, revision, summary, reported_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (execution_id, revision) DO NOTHING
RETURNING id`

const revisionsQuery = `
SELECT s.revision,
       coalesce(sum(r.additions), 0)::integer,
       coalesce(sum(r.deletions), 0)::integer,
       s.reported_at
FROM workspace_execution_snapshots s
LEFT JOIN workspace_execution_snapshot_repositories r ON r.snapshot_id = s.id
WHERE s.execution_id = $1
GROUP BY s.id
ORDER BY s.revision`

type commitRecord struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

type snapshotRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.ExecutionSnapshot {
	return &snapshotRepository{db: db}
}

func commitsOf(commits []entity.SnapshotCommit) (types.JSON, error) {
	records := make([]commitRecord, 0, len(commits))

	for _, commit := range commits {
		records = append(records, commitRecord(commit))
	}

	encoded, err := json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot commits: %w", err)
	}

	return encoded, nil
}

func repositoryOf(model *dbpostgres.WorkspaceExecutionSnapshotRepository) (entity.SnapshotRepository, error) {
	var records []commitRecord

	if err := json.Unmarshal(model.Commits, &records); err != nil {
		return entity.SnapshotRepository{}, fmt.Errorf("read snapshot commits: %w", err)
	}

	commits := make([]entity.SnapshotCommit, 0, len(records))

	for _, record := range records {
		commits = append(commits, entity.SnapshotCommit(record))
	}

	artifactID := uuid.Nil

	if model.DiffArtifactID.Valid {
		parsed, err := uuid.Parse(model.DiffArtifactID.String)
		if err != nil {
			return entity.SnapshotRepository{}, fmt.Errorf("parse snapshot diff artifact id: %w", err)
		}

		artifactID = parsed
	}

	return entity.SnapshotRepository{
		Repository:     model.Repository,
		Branch:         model.Branch,
		BaseSHA:        model.BaseSha,
		HeadSHA:        model.HeadSha,
		Commits:        commits,
		Additions:      model.Additions,
		Deletions:      model.Deletions,
		FilesChanged:   model.FilesChanged,
		DiffArtifactID: artifactID,
	}, nil
}

func previewOf(model *dbpostgres.WorkspaceExecutionSnapshotPreview) entity.SnapshotPreview {
	return entity.SnapshotPreview{
		Name:    model.Name,
		Service: model.Service,
		Path:    model.Path,
		State:   entity.SnapshotPreviewState(model.State),
		Reason:  model.Reason,
		Port:    model.Port,
	}
}

func artifactOf(id uuid.UUID) null.String {
	if id == uuid.Nil {
		return null.String{}
	}

	return null.StringFrom(id.String())
}

func (r *snapshotRepository) Record(
	ctx context.Context,
	snapshot entity.ExecutionSnapshot,
) (entity.ExecutionSnapshot, error) {
	var id string

	err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		claimRevisionQuery,
		snapshot.ExecutionID,
		snapshot.WorkspaceID.String(),
		snapshot.Revision,
		snapshot.Summary,
		snapshot.ReportedAt,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return r.ByRevision(ctx, snapshot.ExecutionID, snapshot.Revision)
	}

	if err != nil {
		return entity.ExecutionSnapshot{}, fmt.Errorf("record review snapshot: %w", err)
	}

	for _, held := range snapshot.Repositories {
		commits, err := commitsOf(held.Commits)
		if err != nil {
			return entity.ExecutionSnapshot{}, err
		}

		model := &dbpostgres.WorkspaceExecutionSnapshotRepository{
			SnapshotID:     id,
			Repository:     held.Repository,
			Branch:         held.Branch,
			BaseSha:        held.BaseSHA,
			HeadSha:        held.HeadSHA,
			Commits:        commits,
			Additions:      held.Additions,
			Deletions:      held.Deletions,
			FilesChanged:   held.FilesChanged,
			DiffArtifactID: artifactOf(held.DiffArtifactID),
		}

		if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
			return entity.ExecutionSnapshot{}, fmt.Errorf("record snapshot repository: %w", err)
		}
	}

	for _, preview := range snapshot.Previews {
		model := &dbpostgres.WorkspaceExecutionSnapshotPreview{
			SnapshotID: id,
			Name:       preview.Name,
			Service:    preview.Service,
			Path:       preview.Path,
			State:      string(preview.State),
			Reason:     preview.Reason,
			Port:       preview.Port,
		}

		if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
			return entity.ExecutionSnapshot{}, fmt.Errorf("record snapshot preview: %w", err)
		}
	}

	return r.ByRevision(ctx, snapshot.ExecutionID, snapshot.Revision)
}

func (r *snapshotRepository) Latest(
	ctx context.Context,
	executionID string,
) (entity.ExecutionSnapshot, error) {
	return r.one(ctx,
		qm.Where(dbpostgres.WorkspaceExecutionSnapshotColumns.ExecutionID+" = ?", executionID),
		qm.OrderBy(dbpostgres.WorkspaceExecutionSnapshotColumns.Revision+" DESC"),
	)
}

func (r *snapshotRepository) ByRevision(
	ctx context.Context,
	executionID string,
	revision int,
) (entity.ExecutionSnapshot, error) {
	return r.one(ctx,
		qm.Where(dbpostgres.WorkspaceExecutionSnapshotColumns.ExecutionID+" = ?", executionID),
		qm.Where(dbpostgres.WorkspaceExecutionSnapshotColumns.Revision+" = ?", revision),
	)
}

func (r *snapshotRepository) Revisions(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionRevision, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, revisionsQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("list review revisions: %w", err)
	}

	defer func() { _ = rows.Close() }()

	revisions := make([]entity.ExecutionRevision, 0)

	for rows.Next() {
		var revision entity.ExecutionRevision

		if err := rows.Scan(
			&revision.Revision, &revision.Additions, &revision.Deletions, &revision.ReportedAt,
		); err != nil {
			return nil, fmt.Errorf("scan review revision: %w", err)
		}

		revisions = append(revisions, revision)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review revisions: %w", err)
	}

	return revisions, nil
}

func (r *snapshotRepository) one(
	ctx context.Context,
	mods ...qm.QueryMod,
) (entity.ExecutionSnapshot, error) {
	model, err := dbpostgres.WorkspaceExecutionSnapshots(mods...).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionSnapshot{}, entity.ErrExecutionSnapshotNotFound
		}

		return entity.ExecutionSnapshot{}, fmt.Errorf("get review snapshot: %w", err)
	}

	snapshot := entity.ExecutionSnapshot{
		ExecutionID: model.ExecutionID,
		Revision:    model.Revision,
		Summary:     model.Summary,
		ReportedAt:  model.ReportedAt,
		CreatedAt:   model.CreatedAt,
	}

	if snapshot.ID, err = uuid.Parse(model.ID); err != nil {
		return entity.ExecutionSnapshot{}, fmt.Errorf("parse review snapshot id: %w", err)
	}

	if snapshot.WorkspaceID, err = uuid.Parse(model.WorkspaceID); err != nil {
		return entity.ExecutionSnapshot{}, fmt.Errorf("parse review snapshot workspace id: %w", err)
	}

	repositories, err := dbpostgres.WorkspaceExecutionSnapshotRepositories(
		qm.Where(dbpostgres.WorkspaceExecutionSnapshotRepositoryColumns.SnapshotID+" = ?", model.ID),
		qm.OrderBy(dbpostgres.WorkspaceExecutionSnapshotRepositoryColumns.Repository),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return entity.ExecutionSnapshot{}, fmt.Errorf("list snapshot repositories: %w", err)
	}

	for _, held := range repositories {
		converted, err := repositoryOf(held)
		if err != nil {
			return entity.ExecutionSnapshot{}, err
		}

		snapshot.Repositories = append(snapshot.Repositories, converted)
	}

	previews, err := dbpostgres.WorkspaceExecutionSnapshotPreviews(
		qm.Where(dbpostgres.WorkspaceExecutionSnapshotPreviewColumns.SnapshotID+" = ?", model.ID),
		qm.OrderBy(dbpostgres.WorkspaceExecutionSnapshotPreviewColumns.Name),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return entity.ExecutionSnapshot{}, fmt.Errorf("list snapshot previews: %w", err)
	}

	for _, preview := range previews {
		snapshot.Previews = append(snapshot.Previews, previewOf(preview))
	}

	return snapshot, nil
}

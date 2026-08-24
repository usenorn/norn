package executionupload

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	digestUniqueIndex   = "workspace_execution_chunks_digest_key"
	sequenceUniqueIndex = "workspace_execution_chunks_sequence_key"
	artifactUniqueIndex = "workspace_execution_artifacts_digest_key"
)

const chunkCursorsQuery = `
SELECT stream,
       max(sequence),
       count(*),
       coalesce(sum(entries), 0),
       coalesce(sum(bytes), 0)
FROM workspace_execution_chunks
WHERE execution_id = $1
GROUP BY stream
ORDER BY stream`

const uploadedBytesQuery = `
SELECT coalesce((SELECT sum(bytes) FROM workspace_execution_chunks WHERE execution_id = $1), 0)
     + coalesce((SELECT sum(bytes) FROM workspace_execution_artifacts WHERE execution_id = $1), 0)`

const qualifiedChunkColumns = `
       c.id,
       c.execution_id,
       c.workspace_id,
       c.stream,
       c.sequence,
       c.digest,
       c.bytes,
       c.entries,
       c.object_key,
       c.first_at,
       c.last_at,
       c.received_at`

const expiredChunksQuery = `
SELECT` + qualifiedChunkColumns + `
FROM workspace_execution_chunks c
LEFT JOIN workspace_execution_policies p ON p.workspace_id = c.workspace_id
WHERE c.received_at < $1::timestamptz - (coalesce(p.upload_retention_days, $2::int) * interval '1 day')
ORDER BY c.received_at
LIMIT $3::int`

func chunkOf(model *dbpostgres.WorkspaceExecutionChunk) (entity.ExecutionChunk, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.ExecutionChunk{}, fmt.Errorf("parse chunk id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.ExecutionChunk{}, fmt.Errorf("parse workspace id: %w", err)
	}

	return entity.ExecutionChunk{
		ID:          id,
		ExecutionID: model.ExecutionID,
		WorkspaceID: workspaceID,
		Stream:      entity.ExecutionStream(model.Stream),
		Sequence:    model.Sequence,
		Digest:      model.Digest,
		Bytes:       model.Bytes,
		Entries:     model.Entries,
		ObjectKey:   model.ObjectKey,
		FirstAt:     model.FirstAt,
		LastAt:      model.LastAt,
		ReceivedAt:  model.ReceivedAt,
	}, nil
}

func chunksOf(models dbpostgres.WorkspaceExecutionChunkSlice) ([]entity.ExecutionChunk, error) {
	chunks := make([]entity.ExecutionChunk, 0, len(models))

	for _, model := range models {
		chunk, err := chunkOf(model)
		if err != nil {
			return nil, err
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

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

type scanner interface {
	Scan(dest ...any) error
}

func scanChunk(row scanner) (entity.ExecutionChunk, error) {
	var (
		chunk       entity.ExecutionChunk
		chunkID     string
		workspaceID string
		stream      string
	)

	if err := row.Scan(
		&chunkID,
		&chunk.ExecutionID,
		&workspaceID,
		&stream,
		&chunk.Sequence,
		&chunk.Digest,
		&chunk.Bytes,
		&chunk.Entries,
		&chunk.ObjectKey,
		&chunk.FirstAt,
		&chunk.LastAt,
		&chunk.ReceivedAt,
	); err != nil {
		return entity.ExecutionChunk{}, err
	}

	parsedID, err := uuid.Parse(chunkID)
	if err != nil {
		return entity.ExecutionChunk{}, fmt.Errorf("parse chunk id: %w", err)
	}

	parsedWorkspace, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.ExecutionChunk{}, fmt.Errorf("parse workspace id: %w", err)
	}

	chunk.ID = parsedID
	chunk.WorkspaceID = parsedWorkspace
	chunk.Stream = entity.ExecutionStream(stream)

	return chunk, nil
}

func (r *executionUploadRepository) AppendChunk(
	ctx context.Context,
	chunk entity.ExecutionChunk,
) (entity.ExecutionChunk, error) {
	model := &dbpostgres.WorkspaceExecutionChunk{
		ExecutionID: chunk.ExecutionID,
		WorkspaceID: chunk.WorkspaceID.String(),
		Stream:      string(chunk.Stream),
		Sequence:    chunk.Sequence,
		Digest:      chunk.Digest,
		Bytes:       chunk.Bytes,
		Entries:     chunk.Entries,
		ObjectKey:   chunk.ObjectKey,
		FirstAt:     chunk.FirstAt,
		LastAt:      chunk.LastAt,
		ReceivedAt:  chunk.ReceivedAt,
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			switch pgErr.ConstraintName {
			case digestUniqueIndex:
				return entity.ExecutionChunk{}, entity.ErrExecutionUploadRecorded
			case sequenceUniqueIndex:
				return entity.ExecutionChunk{}, entity.ErrExecutionChunkConflict
			}
		}

		return entity.ExecutionChunk{}, fmt.Errorf("append execution chunk: %w", err)
	}

	return chunkOf(model)
}

func (r *executionUploadRepository) Chunk(
	ctx context.Context,
	executionID string,
	stream entity.ExecutionStream,
	digest string,
) (entity.ExecutionChunk, error) {
	model, err := dbpostgres.WorkspaceExecutionChunks(
		dbpostgres.WorkspaceExecutionChunkWhere.ExecutionID.EQ(executionID),
		dbpostgres.WorkspaceExecutionChunkWhere.Stream.EQ(string(stream)),
		dbpostgres.WorkspaceExecutionChunkWhere.Digest.EQ(digest),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionChunk{}, entity.ErrExecutionNotFound
		}

		return entity.ExecutionChunk{}, fmt.Errorf("read execution chunk: %w", err)
	}

	return chunkOf(model)
}

func (r *executionUploadRepository) ListChunks(
	ctx context.Context,
	executionID string,
	stream entity.ExecutionStream,
	page entity.ExecutionChunkPage,
) ([]entity.ExecutionChunk, error) {
	page = page.Normalized()

	models, err := dbpostgres.WorkspaceExecutionChunks(
		dbpostgres.WorkspaceExecutionChunkWhere.ExecutionID.EQ(executionID),
		dbpostgres.WorkspaceExecutionChunkWhere.Stream.EQ(string(stream)),
		dbpostgres.WorkspaceExecutionChunkWhere.Sequence.GT(page.After),
		qm.OrderBy(dbpostgres.WorkspaceExecutionChunkColumns.Sequence),
		qm.Limit(page.Limit),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list execution chunks: %w", err)
	}

	return chunksOf(models)
}

func (r *executionUploadRepository) Cursors(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionStreamCursor, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, chunkCursorsQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("read execution stream cursors: %w", err)
	}

	defer func() { _ = rows.Close() }()

	cursors := make([]entity.ExecutionStreamCursor, 0, len(entity.ExecutionStreams()))

	for rows.Next() {
		var (
			cursor entity.ExecutionStreamCursor
			stream string
		)

		if err := rows.Scan(
			&stream, &cursor.LastSequence, &cursor.Chunks, &cursor.Entries, &cursor.Bytes,
		); err != nil {
			return nil, fmt.Errorf("read execution stream cursor: %w", err)
		}

		cursor.Stream = entity.ExecutionStream(stream)
		cursors = append(cursors, cursor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read execution stream cursors: %w", err)
	}

	return cursors, nil
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

func (r *executionUploadRepository) ExpiredChunks(
	ctx context.Context,
	now time.Time,
	fallbackDays, limit int,
) ([]entity.ExecutionChunk, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, expiredChunksQuery, now, fallbackDays, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired execution chunks: %w", err)
	}

	defer func() { _ = rows.Close() }()

	chunks := make([]entity.ExecutionChunk, 0, limit)

	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return nil, fmt.Errorf("read execution chunk: %w", err)
		}

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list expired execution chunks: %w", err)
	}

	return chunks, nil
}

func (r *executionUploadRepository) DropChunk(ctx context.Context, chunkID uuid.UUID) error {
	if _, err := dbpostgres.WorkspaceExecutionChunks(
		dbpostgres.WorkspaceExecutionChunkWhere.ID.EQ(chunkID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("drop execution chunk: %w", err)
	}

	return nil
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

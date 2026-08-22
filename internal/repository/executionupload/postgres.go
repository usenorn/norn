package executionupload

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
	uniqueViolationCode = "23505"
	digestUniqueIndex   = "workspace_execution_chunks_digest_key"
	sequenceUniqueIndex = "workspace_execution_chunks_sequence_key"
	artifactUniqueIndex = "workspace_execution_artifacts_digest_key"
)

const chunkColumns = `
       id,
       execution_id,
       workspace_id,
       stream,
       sequence,
       digest,
       bytes,
       entries,
       object_key,
       first_at,
       last_at,
       received_at`

const appendChunkQuery = `
WITH inserted AS (
    INSERT INTO workspace_execution_chunks
        (execution_id, workspace_id, stream, sequence, digest, bytes, entries,
         object_key, first_at, last_at, received_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    RETURNING *
)
SELECT` + chunkColumns + `
FROM inserted`

const chunkByDigestQuery = `
SELECT` + chunkColumns + `
FROM workspace_execution_chunks
WHERE execution_id = $1 AND stream = $2 AND digest = $3`

const chunksQuery = `
SELECT` + chunkColumns + `
FROM workspace_execution_chunks
WHERE execution_id = $1 AND stream = $2 AND sequence > $3
ORDER BY sequence
LIMIT $4`

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

const dropChunkQuery = `
DELETE FROM workspace_execution_chunks
WHERE id = $1`

const artifactColumns = `
       id,
       execution_id,
       workspace_id,
       name,
       content_type,
       bytes,
       digest,
       object_key,
       created_at`

const saveArtifactQuery = `
WITH inserted AS (
    INSERT INTO workspace_execution_artifacts
        (id, execution_id, workspace_id, name, content_type, bytes, digest, object_key)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    RETURNING *
)
SELECT` + artifactColumns + `
FROM inserted`

const artifactByIDQuery = `
SELECT` + artifactColumns + `
FROM workspace_execution_artifacts
WHERE execution_id = $1 AND id = $2`

const artifactByDigestQuery = `
SELECT` + artifactColumns + `
FROM workspace_execution_artifacts
WHERE execution_id = $1 AND digest = $2`

const artifactsQuery = `
SELECT` + artifactColumns + `
FROM workspace_execution_artifacts
WHERE execution_id = $1
ORDER BY created_at, id`

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

func scanArtifact(row scanner) (entity.ExecutionArtifact, error) {
	var (
		artifact    entity.ExecutionArtifact
		artifactID  string
		workspaceID string
	)

	if err := row.Scan(
		&artifactID,
		&artifact.ExecutionID,
		&workspaceID,
		&artifact.Name,
		&artifact.ContentType,
		&artifact.Bytes,
		&artifact.Digest,
		&artifact.ObjectKey,
		&artifact.CreatedAt,
	); err != nil {
		return entity.ExecutionArtifact{}, err
	}

	parsedID, err := uuid.Parse(artifactID)
	if err != nil {
		return entity.ExecutionArtifact{}, fmt.Errorf("parse artifact id: %w", err)
	}

	parsedWorkspace, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.ExecutionArtifact{}, fmt.Errorf("parse workspace id: %w", err)
	}

	artifact.ID = parsedID
	artifact.WorkspaceID = parsedWorkspace

	return artifact, nil
}

func (r *executionUploadRepository) AppendChunk(
	ctx context.Context,
	chunk entity.ExecutionChunk,
) (entity.ExecutionChunk, error) {
	appended, err := scanChunk(r.db.Querier(ctx).QueryRowContext(
		ctx,
		appendChunkQuery,
		chunk.ExecutionID,
		chunk.WorkspaceID.String(),
		string(chunk.Stream),
		chunk.Sequence,
		chunk.Digest,
		chunk.Bytes,
		chunk.Entries,
		chunk.ObjectKey,
		chunk.FirstAt,
		chunk.LastAt,
		chunk.ReceivedAt,
	))
	if err != nil {
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

	return appended, nil
}

func (r *executionUploadRepository) Chunk(
	ctx context.Context,
	executionID string,
	stream entity.ExecutionStream,
	digest string,
) (entity.ExecutionChunk, error) {
	chunk, err := scanChunk(r.db.Querier(ctx).QueryRowContext(
		ctx, chunkByDigestQuery, executionID, string(stream), digest,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionChunk{}, entity.ErrExecutionNotFound
		}

		return entity.ExecutionChunk{}, fmt.Errorf("read execution chunk: %w", err)
	}

	return chunk, nil
}

func (r *executionUploadRepository) ListChunks(
	ctx context.Context,
	executionID string,
	stream entity.ExecutionStream,
	page entity.ExecutionChunkPage,
) ([]entity.ExecutionChunk, error) {
	page = page.Normalized()

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, chunksQuery, executionID, string(stream), page.After, page.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list execution chunks: %w", err)
	}

	defer func() { _ = rows.Close() }()

	chunks := make([]entity.ExecutionChunk, 0, page.Limit)

	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return nil, fmt.Errorf("read execution chunk: %w", err)
		}

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list execution chunks: %w", err)
	}

	return chunks, nil
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
	if _, err := r.db.Querier(ctx).ExecContext(ctx, dropChunkQuery, chunkID.String()); err != nil {
		return fmt.Errorf("drop execution chunk: %w", err)
	}

	return nil
}

func (r *executionUploadRepository) SaveArtifact(
	ctx context.Context,
	artifact entity.ExecutionArtifact,
) (entity.ExecutionArtifact, error) {
	saved, err := scanArtifact(r.db.Querier(ctx).QueryRowContext(
		ctx,
		saveArtifactQuery,
		artifact.ID.String(),
		artifact.ExecutionID,
		artifact.WorkspaceID.String(),
		artifact.Name,
		artifact.ContentType,
		artifact.Bytes,
		artifact.Digest,
		artifact.ObjectKey,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == uniqueViolationCode &&
			pgErr.ConstraintName == artifactUniqueIndex {
			return entity.ExecutionArtifact{}, entity.ErrExecutionUploadRecorded
		}

		return entity.ExecutionArtifact{}, fmt.Errorf("save execution artifact: %w", err)
	}

	return saved, nil
}

func (r *executionUploadRepository) Artifact(
	ctx context.Context,
	executionID string,
	artifactID uuid.UUID,
) (entity.ExecutionArtifact, error) {
	artifact, err := scanArtifact(r.db.Querier(ctx).QueryRowContext(
		ctx, artifactByIDQuery, executionID, artifactID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionArtifact{}, entity.ErrExecutionArtifactNotFound
		}

		return entity.ExecutionArtifact{}, fmt.Errorf("read execution artifact: %w", err)
	}

	return artifact, nil
}

func (r *executionUploadRepository) ArtifactByDigest(
	ctx context.Context,
	executionID, digest string,
) (entity.ExecutionArtifact, error) {
	artifact, err := scanArtifact(r.db.Querier(ctx).QueryRowContext(
		ctx, artifactByDigestQuery, executionID, digest,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionArtifact{}, entity.ErrExecutionArtifactNotFound
		}

		return entity.ExecutionArtifact{}, fmt.Errorf("read execution artifact: %w", err)
	}

	return artifact, nil
}

func (r *executionUploadRepository) ListArtifacts(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionArtifact, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, artifactsQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("list execution artifacts: %w", err)
	}

	defer func() { _ = rows.Close() }()

	artifacts := make([]entity.ExecutionArtifact, 0, 8)

	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, fmt.Errorf("read execution artifact: %w", err)
		}

		artifacts = append(artifacts, artifact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list execution artifacts: %w", err)
	}

	return artifacts, nil
}

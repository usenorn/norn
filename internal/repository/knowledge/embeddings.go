package knowledge

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const liveTexts = `
SELECT m.content_hash, m.statement || E'\n' || m.rationale || E'\n' || m.conditions AS text
FROM workspace_memories m
WHERE m.workspace_id = %[2]s AND m.status = ANY($%[1]d::text[])
UNION ALL
SELECT p.content_hash, p.heading || E'\n' || p.body AS text
FROM workspace_doc_passages p
JOIN workspace_docs d ON d.id = p.doc_id AND d.workspace_id = p.workspace_id
WHERE p.workspace_id = %[2]s AND d.status = 'active' AND p.doc_version = d.version`

var staleTextsQuery = `
WITH texts AS (` + fmt.Sprintf(liveTexts, 4, "$1") + `)
SELECT DISTINCT ON (t.content_hash) t.content_hash, t.text
FROM texts t
WHERE NOT EXISTS (
    SELECT 1 FROM workspace_knowledge_embeddings e
    WHERE e.workspace_id = $1 AND e.model = $2 AND e.content_hash = t.content_hash)
ORDER BY t.content_hash
LIMIT $3`

const saveEmbeddingsQuery = `
INSERT INTO workspace_knowledge_embeddings (workspace_id, model, content_hash, embedding)
SELECT $1, $2, saved.content_hash, saved.embedding::vector
FROM unnest($3::bytea[], $4::text[]) AS saved (content_hash, embedding)
ON CONFLICT (workspace_id, model, content_hash) DO NOTHING`

var pruneEmbeddingsQuery = `
DELETE FROM workspace_knowledge_embeddings e
WHERE e.workspace_id = $1
  AND (e.model <> $2 OR e.content_hash NOT IN (SELECT content_hash FROM (` + fmt.Sprintf(liveTexts, 3, "$1") + `) live))`

var pendingWorkspacesQuery = `
SELECT w.id
FROM workspaces w
WHERE EXISTS (
          SELECT 1 FROM workspace_docs d
          WHERE d.workspace_id = w.id
            AND NOT EXISTS (
                SELECT 1 FROM workspace_doc_passages p WHERE p.doc_id = d.id AND p.doc_version = d.version))
   OR EXISTS (
          SELECT 1
          FROM workspace_ai_providers a
          CROSS JOIN LATERAL (` + fmt.Sprintf(liveTexts, 1, "w.id") + `) live
          WHERE a.workspace_id = w.id AND a.embedding_model <> ''
            AND NOT EXISTS (
                SELECT 1 FROM workspace_knowledge_embeddings e
                WHERE e.workspace_id = w.id AND e.model = a.embedding_model
                  AND e.content_hash = live.content_hash))
ORDER BY w.id
LIMIT $2`

type embeddingRepository struct {
	db *postgres.Client
}

func NewEmbeddings(db *postgres.Client) repository.KnowledgeEmbedding {
	return &embeddingRepository{db: db}
}

func embeddedStatuses() []string {
	statuses := entity.KnowledgeEmbeddedStatuses()
	names := make([]string, len(statuses))

	for index, status := range statuses {
		names[index] = string(status)
	}

	return names
}

func vectorLiteral(vector []float32) string {
	var builder strings.Builder

	builder.WriteByte('[')

	for index, value := range vector {
		if index > 0 {
			builder.WriteByte(',')
		}

		builder.WriteString(strconv.FormatFloat(float64(value), 'g', -1, 32))
	}

	builder.WriteByte(']')

	return builder.String()
}

func (r *embeddingRepository) Stale(
	ctx context.Context,
	workspaceID uuid.UUID,
	model string,
	limit int,
) ([]entity.KnowledgeText, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, staleTextsQuery, workspaceID, model, limit, embeddedStatuses())
	if err != nil {
		return nil, fmt.Errorf("list knowledge awaiting embeddings: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var texts []entity.KnowledgeText

	for rows.Next() {
		var text entity.KnowledgeText
		if err := rows.Scan(&text.ContentHash, &text.Text); err != nil {
			return nil, fmt.Errorf("scan knowledge awaiting embeddings: %w", err)
		}

		texts = append(texts, text)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read knowledge awaiting embeddings: %w", err)
	}

	return texts, nil
}

func (r *embeddingRepository) Save(
	ctx context.Context,
	workspaceID uuid.UUID,
	model string,
	embeddings []entity.KnowledgeEmbedding,
) error {
	if len(embeddings) == 0 {
		return nil
	}

	hashes := make([][]byte, len(embeddings))
	vectors := make([]string, len(embeddings))

	for index, embedding := range embeddings {
		hashes[index] = embedding.ContentHash
		vectors[index] = vectorLiteral(embedding.Vector)
	}

	if _, err := r.db.Querier(ctx).ExecContext(ctx, saveEmbeddingsQuery, workspaceID, model, hashes, vectors); err != nil {
		return fmt.Errorf("save knowledge embeddings: %w", err)
	}

	return nil
}

func (r *embeddingRepository) Prune(ctx context.Context, workspaceID uuid.UUID, model string) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, pruneEmbeddingsQuery, workspaceID, model, embeddedStatuses(),
	); err != nil {
		return fmt.Errorf("prune knowledge embeddings: %w", err)
	}

	return nil
}

func (r *embeddingRepository) Pending(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, pendingWorkspacesQuery, embeddedStatuses(), limit)
	if err != nil {
		return nil, fmt.Errorf("list workspaces awaiting knowledge refresh: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var workspaceIDs []uuid.UUID

	for rows.Next() {
		var workspaceID uuid.UUID
		if err := rows.Scan(&workspaceID); err != nil {
			return nil, fmt.Errorf("scan workspace awaiting knowledge refresh: %w", err)
		}

		workspaceIDs = append(workspaceIDs, workspaceID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read workspaces awaiting knowledge refresh: %w", err)
	}

	return workspaceIDs, nil
}

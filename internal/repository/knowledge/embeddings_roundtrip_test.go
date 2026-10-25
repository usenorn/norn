package knowledge

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

const embeddingModel = "text-embedding-3-small"

func staleTexts(ctx context.Context, embeddings *embeddingRepository, workspaceID uuid.UUID) ([]string, error) {
	stale, err := embeddings.Stale(ctx, workspaceID, embeddingModel, entity.KnowledgeStaleBatchSize)
	if err != nil {
		return nil, err
	}

	texts := make([]string, len(stale))
	for index, text := range stale {
		texts[index] = text.Text
	}

	sort.Strings(texts)

	return texts, nil
}

func TestOnlyLiveKnowledgeWithoutAVectorForTheModelIsStale(t *testing.T) {
	withKnowledgeFixture(t, func(ctx context.Context, client *postgres.Client, fixture knowledgeFixture) error {
		embeddings := &embeddingRepository{db: client}

		if _, err := fixture.memory(ctx, client, fixtureMemory{statement: "Deploys go out on Tuesdays", rationale: "QA is staffed"}); err != nil {
			return err
		}

		if _, err := fixture.memory(ctx, client, fixtureMemory{statement: "Old rule", status: "withdrawn"}); err != nil {
			return err
		}

		if _, err := fixture.doc(ctx, client, "Runbook", "workspace", uuid.Nil, fixture.authorID,
			[2]string{"Runbook › Rollback", "Revert the tag."},
			[2]string{"Runbook › Alerts", "Revert the tag."},
		); err != nil {
			return err
		}

		archived, err := fixture.doc(ctx, client, "Archive", "workspace", uuid.Nil, fixture.authorID,
			[2]string{"Archive", "Gone."})
		if err != nil {
			return err
		}

		if _, err := client.Querier(ctx).ExecContext(ctx,
			`UPDATE workspace_docs SET status = 'archived', archived_at = now() WHERE id = $1`, archived); err != nil {
			return err
		}

		texts, err := staleTexts(ctx, embeddings, fixture.workspaceID)
		if err != nil {
			return err
		}

		want := []string{
			"Deploys go out on Tuesdays\nQA is staffed\n",
			"Runbook › Alerts\nRevert the tag.",
			"Runbook › Rollback\nRevert the tag.",
		}
		if !slices.Equal(texts, want) {
			return fmt.Errorf("stale = %q, want %q: withdrawn memories and archived docs are never embedded", texts, want)
		}

		stale, err := embeddings.Stale(ctx, fixture.workspaceID, embeddingModel, entity.KnowledgeStaleBatchSize)
		if err != nil {
			return err
		}

		if err := embeddings.Save(ctx, fixture.workspaceID, embeddingModel, []entity.KnowledgeEmbedding{
			{ContentHash: stale[0].ContentHash, Vector: []float32{0.25, -1}},
		}); err != nil {
			return err
		}

		if err := embeddings.Save(ctx, fixture.workspaceID, embeddingModel, []entity.KnowledgeEmbedding{
			{ContentHash: stale[0].ContentHash, Vector: []float32{9, 9}},
		}); err != nil {
			return fmt.Errorf("saving a vector for text that already has one: %w", err)
		}

		remaining, err := staleTexts(ctx, embeddings, fixture.workspaceID)
		if err != nil {
			return err
		}

		if len(remaining) != 2 || slices.Contains(remaining, stale[0].Text) {
			return fmt.Errorf("after one save, stale = %q, want the other two", remaining)
		}

		if other, err := embeddings.Stale(ctx, fixture.workspaceID, "another-model", entity.KnowledgeStaleBatchSize); err != nil || len(other) != 3 {
			return fmt.Errorf("for another model: %d stale, %v; a vector from one model says nothing about another", len(other), err)
		}

		var stored string
		if err := client.Querier(ctx).QueryRowContext(ctx,
			`SELECT embedding::text FROM workspace_knowledge_embeddings WHERE workspace_id = $1 AND content_hash = $2`,
			fixture.workspaceID, stale[0].ContentHash).Scan(&stored); err != nil {
			return err
		}

		if stored != "[0.25,-1]" {
			return fmt.Errorf("stored vector %s, want the first one kept", stored)
		}

		return nil
	})
}

func TestPruningDropsVectorsOfRetiredTextAndOfOtherModels(t *testing.T) {
	withKnowledgeFixture(t, func(ctx context.Context, client *postgres.Client, fixture knowledgeFixture) error {
		embeddings := &embeddingRepository{db: client}

		kept, err := fixture.memory(ctx, client, fixtureMemory{statement: "Keep the vector"})
		if err != nil {
			return err
		}

		retired, err := fixture.memory(ctx, client, fixtureMemory{statement: "Retire the vector"})
		if err != nil {
			return err
		}

		stale, err := embeddings.Stale(ctx, fixture.workspaceID, embeddingModel, entity.KnowledgeStaleBatchSize)
		if err != nil {
			return err
		}

		var saved []entity.KnowledgeEmbedding
		for _, text := range stale {
			saved = append(saved, entity.KnowledgeEmbedding{ContentHash: text.ContentHash, Vector: []float32{1}})
		}

		if err := embeddings.Save(ctx, fixture.workspaceID, embeddingModel, saved); err != nil {
			return err
		}

		if err := embeddings.Save(ctx, fixture.workspaceID, "retired-model", saved); err != nil {
			return err
		}

		if _, err := client.Querier(ctx).ExecContext(ctx,
			`UPDATE workspace_memories SET status = 'withdrawn' WHERE id = $1`, retired); err != nil {
			return err
		}

		if err := embeddings.Prune(ctx, fixture.workspaceID, embeddingModel); err != nil {
			return err
		}

		var left []string

		rows, err := client.Querier(ctx).QueryContext(ctx, `
SELECT e.model || ':' || coalesce(m.statement, '?')
FROM workspace_knowledge_embeddings e
LEFT JOIN workspace_memories m ON m.content_hash = e.content_hash AND m.workspace_id = e.workspace_id
WHERE e.workspace_id = $1`, fixture.workspaceID)
		if err != nil {
			return err
		}

		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var row string
			if err := rows.Scan(&row); err != nil {
				return err
			}

			left = append(left, row)
		}

		if !slices.Equal(left, []string{embeddingModel + ":Keep the vector"}) {
			return fmt.Errorf("left %q, want only the live memory under the current model (kept %s)", left, kept)
		}

		if err := embeddings.Prune(ctx, fixture.workspaceID, ""); err != nil {
			return err
		}

		var count int
		if err := client.Querier(ctx).QueryRowContext(ctx,
			`SELECT count(*) FROM workspace_knowledge_embeddings WHERE workspace_id = $1`, fixture.workspaceID).Scan(&count); err != nil {
			return err
		}

		if count != 0 {
			return fmt.Errorf("%d vectors survived removing the embedding model", count)
		}

		return nil
	})
}

func TestTheSweepFindsWorkspacesWithOutdatedPassagesOrMissingVectors(t *testing.T) {
	withKnowledgeFixture(t, func(ctx context.Context, client *postgres.Client, fixture knowledgeFixture) error {
		embeddings := &embeddingRepository{db: client}

		pendingHas := func() (bool, error) {
			pending, err := embeddings.Pending(ctx, 1_000_000)
			if err != nil {
				return false, err
			}

			return slices.Contains(pending, fixture.workspaceID), nil
		}

		if _, err := fixture.memory(ctx, client, fixtureMemory{statement: "Deploys go out on Tuesdays"}); err != nil {
			return err
		}

		if pending, err := pendingHas(); err != nil || pending {
			return fmt.Errorf("pending = %v, %v; without an embedding model a memory needs nothing", pending, err)
		}

		if _, err := client.Querier(ctx).ExecContext(ctx, `
INSERT INTO workspace_ai_providers (workspace_id, provider, api_key_sealed, embedding_model)
VALUES ($1, 'openai', '\x01', $2)`, fixture.workspaceID, embeddingModel); err != nil {
			return err
		}

		if pending, err := pendingHas(); err != nil || !pending {
			return fmt.Errorf("pending = %v, %v; a memory without a vector awaits one", pending, err)
		}

		stale, err := embeddings.Stale(ctx, fixture.workspaceID, embeddingModel, entity.KnowledgeStaleBatchSize)
		if err != nil {
			return err
		}

		if err := embeddings.Save(ctx, fixture.workspaceID, embeddingModel, []entity.KnowledgeEmbedding{
			{ContentHash: stale[0].ContentHash, Vector: []float32{1}},
		}); err != nil {
			return err
		}

		if pending, err := pendingHas(); err != nil || pending {
			return fmt.Errorf("pending = %v, %v after every text has a vector", pending, err)
		}

		if _, err := client.Querier(ctx).ExecContext(ctx, `
INSERT INTO workspace_docs (workspace_id, title, content, markdown, visibility)
VALUES ($1, 'Unpassaged', '{"type":"doc"}', 'Body', 'workspace')`, fixture.workspaceID); err != nil {
			return err
		}

		if pending, err := pendingHas(); err != nil || !pending {
			return fmt.Errorf("pending = %v, %v; a doc without passages awaits them", pending, err)
		}

		return nil
	})
}

package knowledgeindex

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type indexes struct {
	docs       repository.Doc
	passages   repository.DocPassage
	embeddings repository.KnowledgeEmbedding
	providers  repository.AIProvider
	models     repository.AIModel
	jobs       repository.JobProducer
	transactor repository.Transactor
}

func New(
	docs repository.Doc,
	passages repository.DocPassage,
	embeddings repository.KnowledgeEmbedding,
	providers repository.AIProvider,
	models repository.AIModel,
	jobs repository.JobProducer,
	transactor repository.Transactor,
) service.KnowledgeIndexes {
	return &indexes{
		docs:       docs,
		passages:   passages,
		embeddings: embeddings,
		providers:  providers,
		models:     models,
		jobs:       jobs,
		transactor: transactor,
	}
}

func (s *indexes) Schedule(ctx context.Context, workspaceID uuid.UUID) {
	postgres.AfterCommit(ctx, func(ctx context.Context) {
		if err := s.jobs.EnqueueKnowledgeRefresh(ctx, entity.KnowledgeRefreshPayload{WorkspaceID: workspaceID}); err != nil {
			logging.From(ctx).WarnContext(
				ctx,
				"scheduling a knowledge refresh failed, so the scheduled sweep will carry this change",
				"workspace_id", workspaceID.String(),
				"error", err.Error(),
			)
		}
	})
}

func (s *indexes) Refresh(ctx context.Context, workspaceID uuid.UUID) error {
	if err := s.rebuildPassages(ctx, workspaceID); err != nil {
		return err
	}

	connection, err := s.providers.Get(ctx, workspaceID)
	if errors.Is(err, entity.ErrAIProviderNotConfigured) {
		return s.embeddings.Prune(ctx, workspaceID, "")
	}

	if err != nil {
		return err
	}

	if !connection.Embeds() {
		return s.embeddings.Prune(ctx, workspaceID, "")
	}

	apiKey, err := s.providers.APIKey(ctx, workspaceID)
	if err != nil {
		return err
	}

	if err := s.embed(ctx, connection, apiKey); err != nil {
		return err
	}

	return s.embeddings.Prune(ctx, workspaceID, connection.EmbeddingModel)
}

func (s *indexes) Sweep(ctx context.Context) error {
	workspaceIDs, err := s.embeddings.Pending(ctx, entity.KnowledgeSweepWorkspaceLimit)
	if err != nil {
		return err
	}

	for _, workspaceID := range workspaceIDs {
		if err := s.jobs.EnqueueKnowledgeRefresh(ctx, entity.KnowledgeRefreshPayload{WorkspaceID: workspaceID}); err != nil {
			return err
		}
	}

	return nil
}

func (s *indexes) rebuildPassages(ctx context.Context, workspaceID uuid.UUID) error {
	rebuilt := map[uuid.UUID]bool{}

	for {
		docIDs, err := s.passages.Outdated(ctx, workspaceID, entity.KnowledgePassageBatchSize)
		if err != nil {
			return err
		}

		if len(docIDs) == 0 {
			return nil
		}

		for _, docID := range docIDs {
			if rebuilt[docID] {
				return fmt.Errorf("doc %s still has outdated passages after they were rebuilt", docID)
			}

			rebuilt[docID] = true

			if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
				doc, err := s.docs.LockByID(ctx, workspaceID, docID)
				if err != nil {
					return err
				}

				return s.passages.Replace(ctx, doc)
			}); err != nil {
				return err
			}
		}
	}
}

func (s *indexes) embed(ctx context.Context, connection entity.AIProviderConnection, apiKey string) error {
	embedded := map[string]bool{}

	for {
		texts, err := s.embeddings.Stale(
			ctx, connection.WorkspaceID, connection.EmbeddingModel, entity.KnowledgeStaleBatchSize,
		)
		if err != nil {
			return err
		}

		if len(texts) == 0 {
			return nil
		}

		for _, text := range texts {
			if embedded[string(text.ContentHash)] {
				return fmt.Errorf("knowledge text %x is still stale after its embedding was saved", text.ContentHash)
			}

			embedded[string(text.ContentHash)] = true
		}

		for start := 0; start < len(texts); start += entity.KnowledgeEmbedBatchSize {
			batch := texts[start:min(start+entity.KnowledgeEmbedBatchSize, len(texts))]

			inputs := make([]string, len(batch))
			for index, text := range batch {
				inputs[index] = text.Text
			}

			vectors, err := s.models.Embed(ctx, connection.Endpoint, apiKey, connection.EmbeddingModel, inputs)
			if err != nil {
				return err
			}

			embeddings := make([]entity.KnowledgeEmbedding, len(batch))
			for index, text := range batch {
				embeddings[index] = entity.KnowledgeEmbedding{ContentHash: text.ContentHash, Vector: vectors[index]}
			}

			if err := s.embeddings.Save(ctx, connection.WorkspaceID, connection.EmbeddingModel, embeddings); err != nil {
				return err
			}
		}
	}
}

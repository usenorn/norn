package knowledgeembedding

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type embeddings struct {
	providers repository.AIProvider
	models    repository.AIModel
	cfg       config.Knowledge
}

func New(providers repository.AIProvider, models repository.AIModel, cfg config.Knowledge) service.KnowledgeEmbeddings {
	return &embeddings{providers: providers, models: models, cfg: cfg}
}

func (s *embeddings) Embed(
	ctx context.Context,
	workspaceID uuid.UUID,
	text string,
) (entity.KnowledgeQueryVector, entity.KnowledgeSemantic, error) {
	connection, err := s.providers.Get(ctx, workspaceID)
	if errors.Is(err, entity.ErrAIProviderNotConfigured) {
		return entity.KnowledgeQueryVector{}, entity.KnowledgeSemanticUnconfigured, nil
	}

	if err != nil {
		return entity.KnowledgeQueryVector{}, "", err
	}

	if !connection.Embeds() {
		return entity.KnowledgeQueryVector{}, entity.KnowledgeSemanticUnconfigured, nil
	}

	apiKey, err := s.providers.APIKey(ctx, connection.WorkspaceID)
	if err != nil {
		return entity.KnowledgeQueryVector{}, entity.KnowledgeSemanticUnavailable, nil
	}

	bounded, cancel := context.WithTimeout(ctx, s.cfg.QueryTimeout)
	defer cancel()

	vectors, err := s.models.Embed(
		bounded, connection.Endpoint, apiKey, connection.EmbeddingModel, []string{strings.TrimSpace(text)},
	)
	if err != nil || len(vectors) != 1 {
		return entity.KnowledgeQueryVector{}, entity.KnowledgeSemanticUnavailable, nil
	}

	return entity.KnowledgeQueryVector{Model: connection.EmbeddingModel, Vector: vectors[0]}, entity.KnowledgeSemanticUsed, nil
}

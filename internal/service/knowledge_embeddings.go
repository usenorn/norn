package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=knowledge_embeddings.go -destination=knowledgeembedding/mock_knowledge_embeddings.go -package=knowledgeembedding -mock_names=KnowledgeEmbeddings=MockKnowledgeEmbeddings

type KnowledgeEmbeddings interface {
	Embed(
		ctx context.Context,
		workspaceID uuid.UUID,
		text string,
	) (entity.KnowledgeQueryVector, entity.KnowledgeSemantic, error)
}

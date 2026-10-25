package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=knowledge_embedding.go -destination=knowledge/mock_knowledge_embedding.go -package=knowledge -mock_names=KnowledgeEmbedding=MockKnowledgeEmbedding

type KnowledgeEmbedding interface {
	Stale(ctx context.Context, workspaceID uuid.UUID, model string, limit int) ([]entity.KnowledgeText, error)
	Save(ctx context.Context, workspaceID uuid.UUID, model string, embeddings []entity.KnowledgeEmbedding) error
	Prune(ctx context.Context, workspaceID uuid.UUID, model string) error
	Pending(ctx context.Context, limit int) ([]uuid.UUID, error)
}

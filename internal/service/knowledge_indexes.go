package service

import (
	"context"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source=knowledge_indexes.go -destination=knowledgeindex/mock_knowledge_indexes.go -package=knowledgeindex -mock_names=KnowledgeIndexes=MockKnowledgeIndexes

type KnowledgeIndexes interface {
	Schedule(ctx context.Context, workspaceID uuid.UUID)
	Refresh(ctx context.Context, workspaceID uuid.UUID) error
	Sweep(ctx context.Context) error
}

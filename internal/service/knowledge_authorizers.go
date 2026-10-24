package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=knowledge_authorizers.go -destination=knowledgeauthorizer/mock_knowledge_authorizers.go -package=knowledgeauthorizer -mock_names=KnowledgeAuthorizer=MockKnowledgeAuthorizer

type KnowledgeAuthorizer interface {
	Reader(ctx context.Context, workspaceID uuid.UUID) (entity.KnowledgeDecision, error)
	Read(ctx context.Context, access entity.KnowledgeAccess) (entity.KnowledgeDecision, error)
	Author(
		ctx context.Context,
		workspaceID uuid.UUID,
		audience entity.KnowledgeAudience,
	) (entity.KnowledgeDecision, error)
	Change(ctx context.Context, access entity.KnowledgeAccess) (entity.KnowledgeDecision, error)
	Reshare(
		ctx context.Context,
		access entity.KnowledgeAccess,
		to entity.KnowledgeAudience,
	) (entity.KnowledgeDecision, error)
}

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=knowledge.go -destination=knowledge/mock_knowledge.go -package=knowledge -mock_names=Knowledge=MockKnowledge

type KnowledgeQuery struct {
	Query            entity.SearchQuery
	MemoryKinds      []entity.MemoryKind
	Statuses         []entity.MemoryStatus
	On               time.Time
	ProjectID        *uuid.UUID
	Component        string
	AllProjects      bool
	ProjectIDs       []uuid.UUID
	PersonalAuthorID uuid.UUID
	Limit            int
}

type KnowledgeVector struct {
	Model         string
	Vector        []float32
	MinSimilarity float64
}

type Knowledge interface {
	LexicalMemories(ctx context.Context, workspaceID uuid.UUID, query KnowledgeQuery) ([]entity.KnowledgeCandidate, error)
	LexicalPassages(ctx context.Context, workspaceID uuid.UUID, query KnowledgeQuery) ([]entity.KnowledgeCandidate, error)
	SemanticMemories(
		ctx context.Context,
		workspaceID uuid.UUID,
		query KnowledgeQuery,
		vector KnowledgeVector,
	) ([]entity.KnowledgeCandidate, error)
	SemanticPassages(
		ctx context.Context,
		workspaceID uuid.UUID,
		query KnowledgeQuery,
		vector KnowledgeVector,
	) ([]entity.KnowledgeCandidate, error)
}

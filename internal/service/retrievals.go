package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=retrievals.go -destination=retrieval/mock_retrievals.go -package=retrieval -mock_names=Retrievals=MockRetrievals

type RetrieveInput struct {
	Query           string
	Kinds           []entity.KnowledgeSubjectKind
	MemoryKinds     []entity.MemoryKind
	ProjectID       *uuid.UUID
	Component       string
	IncludeProposed bool
	AsOf            *time.Time
	Limit           int
}

type EnumerateInput struct {
	MemoryKinds     []entity.MemoryKind
	ProjectID       *uuid.UUID
	Component       string
	IncludeProposed bool
	Cursor          string
	Limit           int
}

type KnowledgeList struct {
	Memories   []entity.Memory
	NextCursor string
	Scope      entity.KnowledgeScope
}

type Retrievals interface {
	Retrieve(ctx context.Context, workspaceID uuid.UUID, input RetrieveInput) (entity.KnowledgeRetrieval, error)
	Enumerate(ctx context.Context, workspaceID uuid.UUID, input EnumerateInput) (KnowledgeList, error)
}

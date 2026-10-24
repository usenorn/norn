package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=memoryrevision.go -destination=memoryrevision/mock_memoryrevision.go -package=memoryrevision -mock_names=MemoryRevision=MockMemoryRevision

type MemoryRevision interface {
	Record(ctx context.Context, revision entity.MemoryRevision) error
	List(
		ctx context.Context,
		workspaceID, memoryID uuid.UUID,
		page entity.MemoryRevisionPage,
	) ([]entity.MemoryRevisionSummary, error)
	GetByID(ctx context.Context, workspaceID, memoryID, revisionID uuid.UUID) (entity.MemoryRevision, error)
}

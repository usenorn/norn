package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=memory.go -destination=memory/mock_memory.go -package=memory -mock_names=Memory=MockMemory

type MemoryFilter struct {
	Kinds            []entity.MemoryKind
	Statuses         []entity.MemoryStatus
	AppliesProjectID *uuid.UUID
	Component        string
	WorkspaceWide    bool
	InEffectOn       *time.Time
	AllProjects      bool
	ProjectIDs       []uuid.UUID
	PersonalAuthorID uuid.UUID
}

type Memory interface {
	Create(ctx context.Context, memory entity.Memory) error
	GetByID(ctx context.Context, workspaceID, memoryID uuid.UUID) (entity.Memory, error)
	LockByID(ctx context.Context, workspaceID, memoryID uuid.UUID) (entity.Memory, error)
	Update(ctx context.Context, memory entity.Memory, expectedVersion int) error
	List(
		ctx context.Context,
		workspaceID uuid.UUID,
		filter MemoryFilter,
		page entity.MemoryPage,
	) ([]entity.Memory, error)
	ListByIDs(ctx context.Context, workspaceID uuid.UUID, memoryIDs []uuid.UUID) ([]entity.Memory, error)
	Lineage(ctx context.Context, workspaceID, memoryID uuid.UUID, depth int) ([]entity.Memory, error)
	Enumerate(
		ctx context.Context,
		workspaceID uuid.UUID,
		filter MemoryFilter,
		after *uuid.UUID,
		limit int,
	) ([]entity.Memory, error)
}

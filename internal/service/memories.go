package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=memories.go -destination=memory/mock_memories.go -package=memory -mock_names=Memories=MockMemories

type MemoryContent struct {
	Kind           entity.MemoryKind
	Statement      string
	Rationale      string
	Conditions     string
	Applicability  entity.KnowledgeApplicability
	Basis          entity.MemoryBasis
	Evidence       []entity.MemoryEvidence
	EffectiveFrom  *time.Time
	EffectiveUntil *time.Time
}

type CreateMemoryInput struct {
	MemoryContent
	Audience entity.KnowledgeAudience
}

type CorrectMemoryInput struct {
	MemoryContent
	ExpectedVersion int
}

type SupersedeMemoryInput struct {
	Replacement     CreateMemoryInput
	ExpectedVersion int
}

type ReshareMemoryInput struct {
	ExpectedVersion int
	Audience        entity.KnowledgeAudience
}

type MemoryWrite struct {
	Memory  entity.Memory
	Related []entity.RelatedMemory
	Choices []entity.MemoryChoice
}

type AcceptMemoryInput struct {
	ExpectedVersion int
	Content         *MemoryContent
}

type RejectMemoryInput struct {
	ExpectedVersion int
	Reason          string
}

type MemoryTarget struct {
	ID              uuid.UUID
	ExpectedVersion int
}

type ReconcileMemoryInput struct {
	Resolution      entity.MemoryResolution
	ExpectedVersion int
	Target          *MemoryTarget
}

type MemoryReconciliation struct {
	Memory entity.Memory
	Target *entity.Memory
}

type ListMemoriesInput struct {
	Kinds        []entity.MemoryKind
	Statuses     []entity.MemoryStatus
	Visibilities []entity.KnowledgeVisibility
	Reviewable   bool
	ProjectID    *uuid.UUID
	Component    string
	Cursor       string
	Limit        int
}

type MemoryList struct {
	Memories   []entity.Memory
	NextCursor string
}

type ListMemoryRevisionsInput struct {
	Cursor string
	Limit  int
}

type MemoryHistory struct {
	Revisions  []entity.MemoryRevisionSummary
	NextCursor string
}

type Memories interface {
	Create(ctx context.Context, workspaceID uuid.UUID, input CreateMemoryInput) (MemoryWrite, error)
	Get(ctx context.Context, workspaceID, memoryID uuid.UUID) (entity.Memory, error)
	Related(ctx context.Context, workspaceID, memoryID uuid.UUID) (MemoryWrite, error)
	Lineage(ctx context.Context, workspaceID, memoryID uuid.UUID) ([]entity.Memory, error)
	List(ctx context.Context, workspaceID uuid.UUID, input ListMemoriesInput) (MemoryList, error)
	Correct(ctx context.Context, workspaceID, memoryID uuid.UUID, input CorrectMemoryInput) (entity.Memory, error)
	Supersede(
		ctx context.Context,
		workspaceID, memoryID uuid.UUID,
		input SupersedeMemoryInput,
	) (entity.Memory, error)
	Reconcile(
		ctx context.Context,
		workspaceID, memoryID uuid.UUID,
		input ReconcileMemoryInput,
	) (MemoryReconciliation, error)
	Accept(ctx context.Context, workspaceID, memoryID uuid.UUID, input AcceptMemoryInput) (entity.Memory, error)
	Reject(ctx context.Context, workspaceID, memoryID uuid.UUID, input RejectMemoryInput) (entity.Memory, error)
	Withdraw(ctx context.Context, workspaceID, memoryID uuid.UUID, expectedVersion int) (entity.Memory, error)
	Reshare(ctx context.Context, workspaceID, memoryID uuid.UUID, input ReshareMemoryInput) (entity.Memory, error)
	Revisions(
		ctx context.Context,
		workspaceID, memoryID uuid.UUID,
		input ListMemoryRevisionsInput,
	) (MemoryHistory, error)
	Revision(ctx context.Context, workspaceID, memoryID, revisionID uuid.UUID) (entity.MemoryRevision, error)
}

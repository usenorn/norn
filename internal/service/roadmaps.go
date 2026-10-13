package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=roadmaps.go -destination=roadmap/mock_roadmaps.go -package=roadmap -mock_names=Roadmaps=MockRoadmaps,PublicRoadmaps=MockPublicRoadmaps

type RoadmapEntryView struct {
	Entry     entity.RoadmapEntry
	Reachable bool
	Published bool
	Withheld  bool
}

type RoadmapDraftView struct {
	Roadmap entity.Roadmap
	Entries []RoadmapEntryView
}

type UpdateRoadmapInput struct {
	ExpectedVersion int
	Title           string
	Description     string
	Visibility      entity.RoadmapVisibility
}

type RoadmapEntryContent struct {
	PublicTitle       string
	PublicDescription string
	Status            entity.PublicStatus
	Timeline          entity.RoadmapTimeline
	ProgressMode      entity.RoadmapProgressMode
	ProgressPercent   int
	Visibility        entity.RoadmapVisibility
	Position          int
}

type AddRoadmapEntryInput struct {
	ExpectedVersion int
	Kind            entity.RoadmapEntryKind
	SourceID        uuid.UUID
	Content         RoadmapEntryContent
}

type UpdateRoadmapEntryInput struct {
	ExpectedVersion int
	Content         RoadmapEntryContent
}

type Roadmaps interface {
	Get(ctx context.Context, workspaceID uuid.UUID) (RoadmapDraftView, error)
	Update(ctx context.Context, workspaceID uuid.UUID, input UpdateRoadmapInput) (RoadmapDraftView, error)
	AddEntry(ctx context.Context, workspaceID uuid.UUID, input AddRoadmapEntryInput) (RoadmapDraftView, error)
	UpdateEntry(
		ctx context.Context,
		workspaceID, entryID uuid.UUID,
		input UpdateRoadmapEntryInput,
	) (RoadmapDraftView, error)
	RefreshEntry(ctx context.Context, workspaceID, entryID uuid.UUID, expectedVersion int) (RoadmapDraftView, error)
	RemoveEntry(ctx context.Context, workspaceID, entryID uuid.UUID, expectedVersion int) (RoadmapDraftView, error)
	Publish(ctx context.Context, workspaceID uuid.UUID, expectedVersion int) (RoadmapDraftView, error)
	Unpublish(ctx context.Context, workspaceID uuid.UUID) (RoadmapDraftView, error)
}

type PublicRoadmaps interface {
	Get(ctx context.Context, host string) (entity.PublicRoadmap, error)
	Entry(ctx context.Context, host, publicID string) (entity.PublicRoadmapItem, error)
}

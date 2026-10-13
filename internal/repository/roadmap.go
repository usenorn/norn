package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=roadmap.go -destination=roadmap/mock_roadmap.go -package=roadmap -mock_names=Roadmap=MockRoadmap

type Roadmap interface {
	Ensure(ctx context.Context, workspaceID uuid.UUID) (entity.Roadmap, error)
	Lock(ctx context.Context, workspaceID uuid.UUID) (entity.Roadmap, error)
	Save(ctx context.Context, roadmap entity.Roadmap) (entity.Roadmap, error)
	ListEntries(ctx context.Context, roadmapID uuid.UUID) ([]entity.RoadmapEntry, error)
	GetEntry(ctx context.Context, roadmapID, entryID uuid.UUID) (entity.RoadmapEntry, error)
	CreateEntry(ctx context.Context, entry entity.RoadmapEntry) (entity.RoadmapEntry, error)
	UpdateEntry(ctx context.Context, entry entity.RoadmapEntry) (entity.RoadmapEntry, error)
	DeleteEntry(ctx context.Context, roadmapID, entryID uuid.UUID) error
	EntrySources(ctx context.Context, roadmapID uuid.UUID) (map[uuid.UUID]entity.RoadmapSource, error)
	ListPublishedEntries(ctx context.Context, roadmapID uuid.UUID) ([]entity.PublishedRoadmapEntry, error)
	Publish(
		ctx context.Context,
		roadmapID uuid.UUID,
		publication entity.RoadmapPublication,
		entries []entity.PublishedRoadmapEntry,
	) error
	Unpublish(ctx context.Context, roadmapID uuid.UUID) error
	DeletePublishedEntry(ctx context.Context, roadmapID uuid.UUID, publicID string) error
	PublishedBySlug(ctx context.Context, workspaceSlug string) (entity.PublishedRoadmap, error)
}

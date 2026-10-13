package roadmap

import (
	"context"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type publicRoadmapsService struct {
	roadmaps repository.Roadmap
	domain   string
}

func NewPublic(roadmaps repository.Roadmap, cfg config.Roadmaps) service.PublicRoadmaps {
	return &publicRoadmapsService{roadmaps: roadmaps, domain: cfg.Domain}
}

func (s *publicRoadmapsService) Get(ctx context.Context, host string) (entity.PublicRoadmap, error) {
	slug, ok := entity.RoadmapSlugFromHost(host, s.domain)
	if !ok {
		return entity.PublicRoadmap{}, entity.ErrRoadmapNotFound
	}

	published, err := s.roadmaps.PublishedBySlug(ctx, slug)
	if err != nil {
		return entity.PublicRoadmap{}, err
	}

	return entity.NewPublicRoadmap(published), nil
}

func (s *publicRoadmapsService) Entry(ctx context.Context, host, publicID string) (entity.PublicRoadmapItem, error) {
	if !entity.ValidRoadmapPublicID(publicID) {
		return entity.PublicRoadmapItem{}, entity.ErrRoadmapEntryNotFound
	}

	roadmap, err := s.Get(ctx, host)
	if err != nil {
		return entity.PublicRoadmapItem{}, err
	}

	item, ok := roadmap.Item(publicID)
	if !ok {
		return entity.PublicRoadmapItem{}, entity.ErrRoadmapEntryNotFound
	}

	return item, nil
}

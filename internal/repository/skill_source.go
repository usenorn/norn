package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=skill_source.go -destination=skillsource/mock_skill_source.go -package=skillsource -mock_names=SkillSource=MockSkillSource

type SkillSource interface {
	Discover(ctx context.Context, location entity.AgentSkillLocation) (entity.AgentSkillDiscovery, error)
	Fetch(ctx context.Context, repository, revision, path string) (entity.AgentSkillBundle, error)
}

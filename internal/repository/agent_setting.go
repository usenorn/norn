package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agent_setting.go -destination=agentsetting/mock_agent_setting.go -package=agentsetting -mock_names=AgentSetting=MockAgentSetting

type AgentSetting interface {
	Settings(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.AgentSettings, error)
	Upsert(ctx context.Context, settings entity.AgentSettings) (entity.AgentSettings, error)
}

package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workspace_agent_setting.go -destination=workspaceagentsetting/mock_workspace_agent_setting.go -package=workspaceagentsetting -mock_names=WorkspaceAgentSetting=MockWorkspaceAgentSetting

type WorkspaceAgentSetting interface {
	Settings(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceAgentSettings, error)
	Upsert(ctx context.Context, settings entity.WorkspaceAgentSettings) (entity.WorkspaceAgentSettings, error)
}

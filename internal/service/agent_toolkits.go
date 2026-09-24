package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agent_toolkits.go -destination=agentcapability/mock_agent_toolkits.go -package=agentcapability -mock_names=AgentToolkits=MockAgentToolkits

type AgentToolkits interface {
	Resolve(ctx context.Context, workspaceID, agentID uuid.UUID) (entity.AgentToolkit, error)
}

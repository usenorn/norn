package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agent_mcp_server.go -destination=agentmcpserver/mock_agent_mcp_server.go -package=agentmcpserver -mock_names=AgentMCPServer=MockAgentMCPServer

type AgentMCPServer interface {
	Create(ctx context.Context, server entity.AgentMCPServer, secrets entity.AgentMCPSecrets) (entity.AgentMCPServer, error)
	Update(ctx context.Context, server entity.AgentMCPServer, secrets entity.AgentMCPSecrets) (entity.AgentMCPServer, error)
	Get(ctx context.Context, workspaceID, serverID uuid.UUID) (entity.AgentMCPServer, error)
	Secrets(ctx context.Context, workspaceID, serverID uuid.UUID) (entity.AgentMCPSecrets, error)
	ListByAgent(ctx context.Context, workspaceID, agentID uuid.UUID) ([]entity.AgentMCPServer, error)
	ListLibrary(ctx context.Context, workspaceID uuid.UUID) ([]entity.AgentMCPServer, error)
	CountOwned(ctx context.Context, workspaceID uuid.UUID, agentID *uuid.UUID) (int, error)
	Delete(ctx context.Context, workspaceID, serverID uuid.UUID) error
	Attach(ctx context.Context, attachment entity.AgentCapabilityAttachment) error
	Detach(ctx context.Context, workspaceID, agentID, serverID uuid.UUID) error
	AttachedAgents(ctx context.Context, workspaceID uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
}

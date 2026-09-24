package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agent_mcp_connection.go -destination=agentmcpconnection/mock_agent_mcp_connection.go -package=agentmcpconnection -mock_names=AgentMCPConnection=MockAgentMCPConnection

type AgentMCPConnection interface {
	Get(ctx context.Context, workspaceID, serverID uuid.UUID) (entity.AgentMCPConnection, error)
	List(ctx context.Context, workspaceID uuid.UUID, serverIDs []uuid.UUID) ([]entity.AgentMCPConnection, error)
	Tokens(ctx context.Context, workspaceID, serverID uuid.UUID) (entity.AgentMCPTokens, error)
	Save(ctx context.Context, connection entity.AgentMCPConnection, tokens entity.AgentMCPTokens) (entity.AgentMCPConnection, error)
	MarkFailed(ctx context.Context, workspaceID, serverID uuid.UUID, failure entity.AgentMCPConnectionFailure, at time.Time) error
	Delete(ctx context.Context, workspaceID, serverID uuid.UUID) error
}

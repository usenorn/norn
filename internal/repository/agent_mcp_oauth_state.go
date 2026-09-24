package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agent_mcp_oauth_state.go -destination=agentmcpoauthstate/mock_agent_mcp_oauth_state.go -package=agentmcpoauthstate -mock_names=AgentMCPOAuthState=MockAgentMCPOAuthState

type AgentMCPOAuthState interface {
	Put(ctx context.Context, state string, attempt entity.AgentMCPOAuthState) error
	Take(ctx context.Context, state string) (entity.AgentMCPOAuthState, error)
}

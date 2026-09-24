package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=mcp_authorizer.go -destination=mcpauthorizer/mock_mcp_authorizer.go -package=mcpauthorizer -mock_names=MCPAuthorizer=MockMCPAuthorizer

type MCPAuthorizer interface {
	Discover(ctx context.Context, serverURL string, allowPrivate bool) (entity.AgentMCPAuthServer, error)
	Register(ctx context.Context, server entity.AgentMCPAuthServer, redirectURI string, allowPrivate bool) (entity.AgentMCPClient, error)
	AuthorizationURL(server entity.AgentMCPAuthServer, client entity.AgentMCPClient, redirectURI, state, verifier string) string
	Exchange(ctx context.Context, attempt entity.AgentMCPOAuthState, code, redirectURI string, allowPrivate bool) (entity.AgentMCPTokens, error)
	Refresh(ctx context.Context, connection entity.AgentMCPConnection, tokens entity.AgentMCPTokens, allowPrivate bool) (entity.AgentMCPTokens, error)
}

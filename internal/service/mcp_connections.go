package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=mcp_connections.go -destination=mcpconnection/mock_mcp_connections.go -package=mcpconnection -mock_names=MCPConnections=MockMCPConnections

type MCPConnections interface {
	RegisterClient(ctx context.Context, input RegisterMCPClientInput) (entity.MCPClient, error)
	BeginAuthorization(ctx context.Context, input BeginMCPAuthorizationInput) (string, error)
	DescribeAuthorization(ctx context.Context, requestID string) (MCPAuthorizationView, error)
	Approve(ctx context.Context, requestID string) (MCPAuthorizationDecision, error)
	Deny(ctx context.Context, requestID string) (MCPAuthorizationDecision, error)
	Exchange(ctx context.Context, input ExchangeMCPCodeInput) (MCPTokenPair, error)
	Refresh(ctx context.Context, input RefreshMCPTokenInput) (MCPTokenPair, error)
	RevokeByValue(ctx context.Context, token, clientID string) error
	Authenticate(ctx context.Context, token string) (entity.Actor, entity.MCPConnection, error)
	List(ctx context.Context) ([]entity.MCPConnection, error)
	Revoke(ctx context.Context, connectionID uuid.UUID) error
	Narrow(ctx context.Context, connectionID uuid.UUID, input NarrowMCPConnectionInput) (entity.MCPConnection, error)
	ListForWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]OwnedMCPConnection, error)
	RevokeInWorkspace(ctx context.Context, workspaceID, connectionID uuid.UUID) error
}

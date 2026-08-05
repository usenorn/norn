package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=mcp_connection.go -destination=mcpconnection/mock_mcp_connection.go -package=mcpconnection -mock_names=MCPConnection=MockMCPConnection

type MCPConnection interface {
	Create(ctx context.Context, connection entity.MCPConnection) (entity.MCPConnection, error)
	GetByID(ctx context.Context, connectionID uuid.UUID) (entity.MCPConnection, error)
	ListByAccount(ctx context.Context, accountID uuid.UUID) ([]entity.MCPConnection, error)
	ListReachingWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]entity.MCPConnection, error)
	Revoke(ctx context.Context, connectionID uuid.UUID, revokedAt time.Time) error
	RecordUsage(ctx context.Context, connectionID uuid.UUID, usedAt time.Time) error
	ReplaceGrants(ctx context.Context, connectionID uuid.UUID, grants entity.APITokenGrants) error
	SetScopes(ctx context.Context, connectionID uuid.UUID, scopes entity.APIScopeSet) error
}

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=mcp_token.go -destination=mcptoken/mock_mcp_token.go -package=mcptoken -mock_names=MCPToken=MockMCPToken

type MCPToken interface {
	Create(ctx context.Context, token entity.MCPToken) (entity.MCPToken, error)
	GetByHash(ctx context.Context, tokenHash []byte) (entity.MCPToken, error)
	Consume(ctx context.Context, tokenID uuid.UUID, consumedAt time.Time) error
	DeleteForConnection(ctx context.Context, connectionID uuid.UUID) error
	PruneExpired(ctx context.Context, connectionID uuid.UUID, before time.Time) error
}

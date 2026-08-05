package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=mcp_client.go -destination=mcpclient/mock_mcp_client.go -package=mcpclient -mock_names=MCPClient=MockMCPClient

type MCPClient interface {
	Create(ctx context.Context, client entity.MCPClient) (entity.MCPClient, error)
	GetByID(ctx context.Context, clientID uuid.UUID) (entity.MCPClient, error)
}

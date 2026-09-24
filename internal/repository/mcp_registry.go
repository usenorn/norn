package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=mcp_registry.go -destination=mcpregistry/mock_mcp_registry.go -package=mcpregistry -mock_names=MCPRegistry=MockMCPRegistry

type MCPRegistry interface {
	Search(ctx context.Context, query string) ([]entity.AgentMCPRegistryEntry, error)
}

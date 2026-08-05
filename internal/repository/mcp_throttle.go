package repository

import "context"

//go:generate go tool mockgen -source=mcp_throttle.go -destination=mcpthrottle/mock_mcp_throttle.go -package=mcpthrottle -mock_names=MCPThrottle=MockMCPThrottle

type MCPThrottle interface {
	Record(ctx context.Context, key string) (int, error)
}

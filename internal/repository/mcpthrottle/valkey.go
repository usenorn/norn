package mcpthrottle

import (
	"context"
	"fmt"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

type mcpThrottleRepository struct {
	client *valkey.Client
	cfg    config.MCP
}

func New(client *valkey.Client, cfg config.MCP) repository.MCPThrottle {
	return &mcpThrottleRepository{client: client, cfg: cfg}
}

func (r *mcpThrottleRepository) Record(ctx context.Context, key string) (int, error) {
	taken, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("record mcp request: %w", err)
	}

	if taken == 1 {
		if err := r.client.Expire(ctx, key, r.cfg.RateWindow).Err(); err != nil {
			return 0, fmt.Errorf("expire mcp request window: %w", err)
		}
	}

	return int(taken), nil
}

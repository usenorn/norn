package agentthrottle

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

const keyPrefix = "agent-actions:"

type agentThrottleRepository struct {
	client *valkey.Client
}

func New(client *valkey.Client) repository.AgentThrottle {
	return &agentThrottleRepository{client: client}
}

func (r *agentThrottleRepository) Record(ctx context.Context, agentID uuid.UUID) (int, error) {
	key := keyPrefix + agentID.String()

	taken, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("record agent action: %w", err)
	}

	if taken == 1 {
		if err := r.client.Expire(ctx, key, entity.AgentActionWindow).Err(); err != nil {
			return 0, fmt.Errorf("expire agent action window: %w", err)
		}
	}

	return int(taken), nil
}

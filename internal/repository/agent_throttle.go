package repository

import (
	"context"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source=agent_throttle.go -destination=agentthrottle/mock_agent_throttle.go -package=agentthrottle -mock_names=AgentThrottle=MockAgentThrottle

type AgentThrottle interface {
	Record(ctx context.Context, agentID uuid.UUID) (int, error)
}

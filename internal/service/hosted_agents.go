package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=hosted_agents.go -destination=hostedagent/mock_hosted_agents.go -package=hostedagent -mock_names=HostedAgents=MockHostedAgents

type HostedAgents interface {
	Converse(ctx context.Context, workspaceID, agentID uuid.UUID, turns []entity.AgentTurn) (entity.AgentReply, error)
}

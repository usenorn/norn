package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agent_skill.go -destination=agentskill/mock_agent_skill.go -package=agentskill -mock_names=AgentSkill=MockAgentSkill

type AgentSkill interface {
	Create(ctx context.Context, skill entity.AgentSkill) (entity.AgentSkill, error)
	Replace(ctx context.Context, skill entity.AgentSkill) (entity.AgentSkill, error)
	Get(ctx context.Context, workspaceID, skillID uuid.UUID) (entity.AgentSkill, error)
	ListByAgent(ctx context.Context, workspaceID, agentID uuid.UUID) ([]entity.AgentSkill, error)
	ListLibrary(ctx context.Context, workspaceID uuid.UUID) ([]entity.AgentSkill, error)
	CountOwned(ctx context.Context, workspaceID uuid.UUID, agentID *uuid.UUID) (int, error)
	Delete(ctx context.Context, workspaceID, skillID uuid.UUID) error
	Attach(ctx context.Context, attachment entity.AgentCapabilityAttachment) error
	Detach(ctx context.Context, workspaceID, agentID, skillID uuid.UUID) error
	AttachedAgents(ctx context.Context, workspaceID uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
}

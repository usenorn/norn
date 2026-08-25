package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agent.go -destination=agent/mock_agent.go -package=agent -mock_names=Agent=MockAgent

type Agent interface {
	Create(ctx context.Context, agent entity.Agent) (entity.Agent, error)
	GetByID(ctx context.Context, workspaceID, agentID uuid.UUID) (entity.Agent, error)
	GetByAccountID(ctx context.Context, accountID uuid.UUID) (entity.Agent, error)
	ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]entity.Agent, error)
	Disable(ctx context.Context, workspaceID, agentID uuid.UUID, disabledAt time.Time) error
	Enable(ctx context.Context, workspaceID, agentID uuid.UUID) error
}

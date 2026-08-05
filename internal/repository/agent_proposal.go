package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agent_proposal.go -destination=agentproposal/mock_agent_proposal.go -package=agentproposal -mock_names=AgentProposal=MockAgentProposal

type AgentProposal interface {
	Create(ctx context.Context, proposal entity.AgentProposal) (entity.AgentProposal, error)
	GetByID(ctx context.Context, workspaceID, proposalID uuid.UUID) (entity.AgentProposal, error)
	ListWaiting(ctx context.Context, workspaceID uuid.UUID, limit int) ([]entity.AgentProposal, error)
	ListByAgent(ctx context.Context, agentID uuid.UUID, limit int) ([]entity.AgentProposal, error)
	Settle(
		ctx context.Context,
		proposalID uuid.UUID,
		status entity.AgentProposalStatus,
		decidedBy uuid.UUID,
		decidedAt time.Time,
		failure string,
	) error
}

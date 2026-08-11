package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agents.go -destination=agent/mock_agents.go -package=agent -mock_names=Agents=MockAgents

type Agents interface {
	Register(ctx context.Context, input RegisterAgentInput) (RegisteredAgent, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]OwnedAgent, error)
	Get(ctx context.Context, workspaceID, agentID uuid.UUID) (OwnedAgent, error)
	Disable(ctx context.Context, workspaceID, agentID uuid.UUID) error
	Rotate(ctx context.Context, workspaceID, agentID uuid.UUID) (RegisteredAgent, error)
	Activity(ctx context.Context, workspaceID, agentID uuid.UUID, page entity.ActivityPage) (ActivityPage, error)
	Authenticate(ctx context.Context, accountID uuid.UUID, actor entity.Actor) (entity.Actor, error)

	Settings(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.AgentSettings, error)
	Configure(ctx context.Context, input ConfigureAgentInput) (entity.AgentSettings, error)

	Waiting(ctx context.Context, workspaceID uuid.UUID) ([]WaitingProposal, error)
	Approve(ctx context.Context, workspaceID, proposalID uuid.UUID) (entity.AgentProposal, error)
	Reject(ctx context.Context, workspaceID, proposalID uuid.UUID) (entity.AgentProposal, error)
}

type WaitingProposal struct {
	Proposal entity.AgentProposal
	Team     entity.Team
	Issue    entity.Issue
	Checks   IssueChecks
	Proposed []entity.Check
	State    entity.WorkflowState
}

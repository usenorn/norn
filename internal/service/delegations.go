package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=delegations.go -destination=delegation/mock_delegations.go -package=delegation -mock_names=Delegations=MockDelegations

type DelegationTargets struct {
	Agent     entity.Agent
	Placement ExecutionPlacement
}

type Delegations interface {
	Delegate(ctx context.Context, workspaceID, issueID uuid.UUID, input DelegateIssueInput) (entity.IssueDelegation, error)
	Targets(
		ctx context.Context, workspaceID, issueID, agentAccountID uuid.UUID,
	) (DelegationTargets, error)
	Recall(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueDelegation, error)
	History(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueDelegation, error)
}

type DelegateIssueInput struct {
	AgentAccountID uuid.UUID
	Brief          string
	Params         entity.DelegationParams
}

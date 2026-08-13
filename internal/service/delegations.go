package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=delegations.go -destination=delegation/mock_delegations.go -package=delegation -mock_names=Delegations=MockDelegations

type Delegations interface {
	Delegate(ctx context.Context, workspaceID, issueID uuid.UUID, input DelegateIssueInput) (entity.IssueDelegation, error)
	Recall(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueDelegation, error)
	History(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueDelegation, error)
	Queue(ctx context.Context, workspaceID uuid.UUID) ([]DelegatedWork, error)
	Claim(ctx context.Context, workspaceID, issueID uuid.UUID, input ClaimDelegationInput) (entity.IssueDelegation, error)
	Heartbeat(ctx context.Context, workspaceID, issueID uuid.UUID, input HeartbeatDelegationInput) (entity.IssueDelegation, error)
	ReleaseClaim(ctx context.Context, workspaceID, issueID uuid.UUID, token uuid.UUID) (entity.IssueDelegation, error)
}

type DelegateIssueInput struct {
	AgentAccountID uuid.UUID
	Brief          string
}

type DelegatedWork struct {
	Delegation entity.IssueDelegation
	Issue      entity.Issue
}

type ClaimDelegationInput struct {
	Runner string
	TTL    time.Duration
}

type HeartbeatDelegationInput struct {
	Token uuid.UUID
	TTL   time.Duration
}

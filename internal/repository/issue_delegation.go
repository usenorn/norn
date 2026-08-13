package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_delegation.go -destination=issuedelegation/mock_issue_delegation.go -package=issuedelegation -mock_names=IssueDelegation=MockIssueDelegation

type RecallDelegation struct {
	IssueID    uuid.UUID
	AccountID  uuid.UUID
	RecalledAt time.Time
}

type ClaimDelegation struct {
	IssueID   uuid.UUID
	AgentID   uuid.UUID
	Runner    string
	Token     uuid.UUID
	ClaimedAt time.Time
	ExpiresAt time.Time
}

type HeartbeatDelegation struct {
	IssueID   uuid.UUID
	Token     uuid.UUID
	ExpiresAt time.Time
}

type ReleaseDelegation struct {
	IssueID uuid.UUID
	Token   uuid.UUID
}

type IssueDelegation interface {
	Open(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueDelegation, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueDelegation, error)
	ListOpenByAgent(ctx context.Context, workspaceID, agentID uuid.UUID) ([]entity.IssueDelegation, error)
	Delegate(ctx context.Context, delegation entity.IssueDelegation) (entity.IssueDelegation, error)
	Recall(ctx context.Context, workspaceID uuid.UUID, recall RecallDelegation) (entity.IssueDelegation, error)
	Claim(ctx context.Context, workspaceID uuid.UUID, claim ClaimDelegation) (entity.IssueDelegation, error)
	Heartbeat(ctx context.Context, workspaceID uuid.UUID, beat HeartbeatDelegation) (entity.IssueDelegation, error)
	ReleaseClaim(ctx context.Context, workspaceID uuid.UUID, release ReleaseDelegation) (entity.IssueDelegation, error)
}

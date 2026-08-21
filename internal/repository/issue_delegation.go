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

type IssueDelegation interface {
	Open(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueDelegation, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueDelegation, error)
	Delegate(ctx context.Context, delegation entity.IssueDelegation) (entity.IssueDelegation, error)
	Recall(ctx context.Context, workspaceID uuid.UUID, recall RecallDelegation) (entity.IssueDelegation, error)
}

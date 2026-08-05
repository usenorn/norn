package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_follower.go -destination=issuefollower/mock_issue_follower.go -package=issuefollower -mock_names=IssueFollower=MockIssueFollower

type IssueFollower interface {
	Follow(ctx context.Context, follower entity.IssueFollower) error
	SetState(ctx context.Context, follower entity.IssueFollower) error
	Get(ctx context.Context, issueID, accountID uuid.UUID) (entity.IssueFollower, error)
	List(ctx context.Context, issueID uuid.UUID) ([]entity.IssueFollower, error)
}

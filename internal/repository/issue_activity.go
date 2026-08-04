package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_activity.go -destination=issueactivity/mock_issue_activity.go -package=issueactivity -mock_names=IssueActivity=MockIssueActivity

type IssueActivity interface {
	Record(ctx context.Context, activity entity.IssueActivity) error
	ListByIssueID(ctx context.Context, issueID uuid.UUID, page entity.IssueActivityPage) ([]entity.IssueActivity, error)
}

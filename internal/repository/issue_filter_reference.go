package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_filter_reference.go -destination=issuefilterreference/mock_issue_filter_reference.go -package=issuefilterreference -mock_names=IssueFilterReference=MockIssueFilterReference

type IssueFilterReference interface {
	Resolve(
		ctx context.Context,
		workspaceID uuid.UUID,
		scope entity.TeamScope,
		wanted []entity.IssueFilterReference,
	) ([]entity.IssueFilterReference, error)
}

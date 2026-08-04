package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_relation.go -destination=issuerelation/mock_issue_relation.go -package=issuerelation -mock_names=IssueRelation=MockIssueRelation

type IssueRelation interface {
	Create(ctx context.Context, relation entity.StoredIssueRelation) (entity.StoredIssueRelation, error)
	GetByID(ctx context.Context, workspaceID, relationID uuid.UUID) (entity.StoredIssueRelation, error)
	Delete(ctx context.Context, relationID uuid.UUID) error
	ListForIssue(ctx context.Context, workspaceID, issueID uuid.UUID, scope entity.TeamScope) ([]entity.IssueRelation, error)
	FindPair(ctx context.Context, workspaceID, issueID, counterpartID uuid.UUID, scope entity.TeamScope) (entity.IssueRelation, error)
}

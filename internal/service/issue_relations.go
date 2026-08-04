package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_relations.go -destination=issuerelation/mock_issue_relations.go -package=issuerelation -mock_names=IssueRelations=MockIssueRelations

type IssueRelations interface {
	Add(ctx context.Context, workspaceID, issueID uuid.UUID, input AddIssueRelationInput) (entity.IssueRelation, error)
	Remove(ctx context.Context, workspaceID, issueID, relationID uuid.UUID) error
	List(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueRelationGroup, error)
}

type AddIssueRelationInput struct {
	Kind           entity.IssueRelationView
	CounterpartID  uuid.UUID
	CloseDuplicate bool
}

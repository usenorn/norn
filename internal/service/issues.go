package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issues.go -destination=issue/mock_issues.go -package=issue -mock_names=Issues=MockIssues

type Issues interface {
	Create(ctx context.Context, input CreateIssueInput) (entity.Issue, error)
	Get(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.Issue, error)
	GetByReference(ctx context.Context, workspaceID uuid.UUID, reference string) (entity.Issue, error)
	List(ctx context.Context, workspaceID uuid.UUID, input ListIssuesInput) (IssuePage, error)
	Update(ctx context.Context, workspaceID, issueID uuid.UUID, input UpdateIssueInput) (entity.Issue, error)
	SetParent(ctx context.Context, workspaceID, issueID uuid.UUID, input SetIssueParentInput) (entity.Issue, error)
	Children(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.Issue, entity.IssueProgress, error)
	SetStatus(ctx context.Context, workspaceID, issueID uuid.UUID, input SetIssueStatusInput) (entity.Issue, error)
	Purge(ctx context.Context, issueID uuid.UUID) error
	MoveToTeam(ctx context.Context, workspaceID, issueID uuid.UUID, input MoveIssueInput) (entity.Issue, error)
	SetLabels(ctx context.Context, workspaceID, issueID uuid.UUID, input SetIssueLabelsInput) ([]entity.Label, error)
	Progress(ctx context.Context, workspaceID uuid.UUID, input ProgressInput) (entity.IssueProgress, error)
	Activity(ctx context.Context, workspaceID, issueID uuid.UUID, input ListIssueActivityInput) (IssueActivityPage, error)
}

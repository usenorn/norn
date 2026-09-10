package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_template.go -destination=issuetemplate/mock_issue_template.go -package=issuetemplate -mock_names=IssueTemplate=MockIssueTemplate

type IssueTemplate interface {
	Create(ctx context.Context, template entity.IssueTemplate) (entity.IssueTemplate, error)
	Update(ctx context.Context, template entity.IssueTemplate) (entity.IssueTemplate, error)
	ListForTeam(ctx context.Context, workspaceID, teamID uuid.UUID) ([]entity.IssueTemplate, error)
	GetByID(ctx context.Context, workspaceID, templateID uuid.UUID) (entity.IssueTemplate, error)
	CountForTeam(ctx context.Context, workspaceID, teamID uuid.UUID) (int, error)
	Remove(ctx context.Context, workspaceID, templateID uuid.UUID) error
}

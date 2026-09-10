package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_templates.go -destination=issuetemplate/mock_issue_templates.go -package=issuetemplate -mock_names=IssueTemplates=MockIssueTemplates

type SaveIssueTemplateInput struct {
	TemplateID        uuid.UUID
	TeamID            uuid.UUID
	Name              string
	Description       string
	Title             string
	Body              string
	BodyDoc           *entity.Document
	RequiredFields    []entity.TemplateField
	StateID           uuid.UUID
	ProjectID         uuid.UUID
	AssigneeAccountID uuid.UUID
	LabelIDs          []uuid.UUID
	Priority          entity.IssuePriority
	Estimate          int
	Position          int
}

type IssueTemplates interface {
	Save(ctx context.Context, workspaceID uuid.UUID, input SaveIssueTemplateInput) (entity.IssueTemplate, error)
	ListForTeam(ctx context.Context, workspaceID, teamID uuid.UUID) ([]entity.IssueTemplate, error)
	Remove(ctx context.Context, workspaceID, templateID uuid.UUID) error
}

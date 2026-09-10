package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_drafts.go -destination=issuedraft/mock_issue_drafts.go -package=issuedraft -mock_names=IssueDrafts=MockIssueDrafts

type SaveIssueDraftInput struct {
	DraftID           uuid.UUID
	TeamID            uuid.UUID
	Title             string
	Description       string
	DescriptionDoc    *entity.Document
	StateID           uuid.UUID
	ProjectID         uuid.UUID
	CycleID           uuid.UUID
	AssigneeAccountID uuid.UUID
	ParentIssueID     uuid.UUID
	LabelIDs          []uuid.UUID
	AttachmentIDs     []uuid.UUID
	Priority          entity.IssuePriority
	Estimate          int
	DueOn             string
}

type IssueDrafts interface {
	Save(ctx context.Context, workspaceID uuid.UUID, input SaveIssueDraftInput) (entity.IssueDraft, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]entity.IssueDraft, error)
	Remove(ctx context.Context, workspaceID, draftID uuid.UUID) error
}

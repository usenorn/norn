package service

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type CreateIssueInput struct {
	WorkspaceID uuid.UUID
	TeamID      uuid.UUID
	Title       string
}

type ListIssuesInput struct {
	Cursor   string
	Limit    int
	TeamID   *uuid.UUID
	Statuses []entity.IssueStatus
}

type IssuePage struct {
	Issues     []entity.Issue
	NextCursor string
}

type ListIssueActivityInput struct {
	Cursor string
	Limit  int
}

type IssueActivityPage struct {
	Entries    []entity.IssueActivity
	NextCursor string
}

type SetIssueParentInput struct {
	ExpectedVersion int
	ParentID        *uuid.UUID
}

type UpdateIssueInput struct {
	ExpectedVersion         int
	AcknowledgeOpenChildren bool
	Title                   *string
	StateID                 *uuid.UUID
	Description             *string
	Priority                *entity.IssuePriority
	AssigneeID              *uuid.UUID
	Estimate                *int
	DueOn                   *string
	Clear                   []string
}

type MoveIssueInput struct {
	ExpectedVersion      int
	TeamID               uuid.UUID
	AcknowledgeLabelLoss bool
}

type SetIssueStatusInput struct {
	ExpectedVersion int
	Status          entity.IssueStatus
}

type SetIssueLabelsInput struct {
	ExpectedVersion int
	LabelIDs        []uuid.UUID
}

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
	Cursor    string
	Limit     int
	TeamID    *uuid.UUID
	CycleID   *uuid.UUID
	ProjectID *uuid.UUID
	Statuses  []entity.IssueStatus
}

type IssuePage struct {
	Issues     []entity.Issue
	NextCursor string
}

type QueryIssuesInput struct {
	Filter  *entity.IssueFilter
	Sort    []entity.IssueSort
	GroupBy entity.IssueGroupBy
	Cursor  string
	Limit   int
}

type IssueQueryResult struct {
	Issues     []entity.Issue
	NextCursor string
	Groups     []entity.IssueGroupTally
}

type ProgressInput struct {
	TeamID    *uuid.UUID
	CycleID   *uuid.UUID
	ProjectID *uuid.UUID
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
	CycleID                 *uuid.UUID
	ProjectID               *uuid.UUID
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

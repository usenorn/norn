package service

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type CreateIssueInput struct {
	WorkspaceID       uuid.UUID
	TeamID            uuid.UUID
	Title             string
	Description       string
	Priority          entity.IssuePriority
	AssigneeAccountID uuid.UUID
	Estimate          int
	DueOn             string
	StateID           uuid.UUID
	CycleID           uuid.UUID
	ProjectID         uuid.UUID
	LabelIDs          []uuid.UUID
	Origin            *entity.ImportOrigin
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
	Text     string
	Filter   *entity.IssueFilter
	Sort     []entity.IssueSort
	GroupBy  entity.IssueGroupBy
	Cursor   string
	Limit    int
	PerGroup int
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

type ListActivityInput struct {
	Cursor string
	Order  entity.ActivityOrder
	Limit  int
}

type ActivityPage struct {
	Events     []entity.ActivityEvent
	NextCursor string
}

type SetIssueParentInput struct {
	ExpectedVersion int
	ParentID        *uuid.UUID
}

type UpdateIssueInput struct {
	ExpectedVersion           int
	AcknowledgeOpenChildren   bool
	AcknowledgeUnprovenChecks bool
	Title                     *string
	StateID                   *uuid.UUID
	Description               *string
	Priority                  *entity.IssuePriority
	AssigneeID                *uuid.UUID
	Estimate                  *int
	DueOn                     *string
	CycleID                   *uuid.UUID
	ProjectID                 *uuid.UUID
	AfterIssueID              *uuid.UUID
	BeforeIssueID             *uuid.UUID
	Clear                     []string
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

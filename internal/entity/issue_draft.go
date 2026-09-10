package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	IssueDraftMaxPerAccount = 50
	IssueDraftPageSize      = 50
)

var (
	ErrIssueDraftNotFound = errors.New("draft not found")
	ErrTooManyIssueDrafts = errors.New("too many drafts kept for one person in one workspace")
)

type IssueDraft struct {
	ID                uuid.UUID
	WorkspaceID       uuid.UUID
	AccountID         uuid.UUID
	TeamID            uuid.UUID
	Title             string
	Description       string
	DescriptionDoc    Document
	StateID           uuid.UUID
	ProjectID         uuid.UUID
	CycleID           uuid.UUID
	AssigneeAccountID uuid.UUID
	ParentIssueID     uuid.UUID
	LabelIDs          []uuid.UUID
	AttachmentIDs     []uuid.UUID
	Priority          IssuePriority
	Estimate          int
	DueOn             string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (d IssueDraft) Empty() bool {
	return d.Title == "" && d.Description == "" && d.TeamID == uuid.Nil
}

func ValidateIssueDraftTitle(field, title string) FieldError {
	if len([]rune(title)) > IssueTitleMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

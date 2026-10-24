package entity

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	DocLinkFieldIssueID   = "issueId"
	DocLinkFieldProjectID = "projectId"
)

var (
	ErrDocLinkNotFound = errors.New("doc link not found")
	ErrDocLinkExists   = errors.New("doc already links there")
)

type DocLinkTarget string

const (
	DocLinkTargetIssue   DocLinkTarget = "issue"
	DocLinkTargetProject DocLinkTarget = "project"
)

func DocLinkTargets() []DocLinkTarget {
	return []DocLinkTarget{DocLinkTargetIssue, DocLinkTargetProject}
}

func (t DocLinkTarget) Valid() bool {
	return slices.Contains(DocLinkTargets(), t)
}

func DocLinkTargetOf(issueID, projectID uuid.UUID) (DocLinkTarget, error) {
	switch {
	case issueID != uuid.Nil && projectID != uuid.Nil:
		return "", NewValidationError(FieldError{Field: DocLinkFieldProjectID, Code: ValidationCodeUnsupportedValue})
	case issueID != uuid.Nil:
		return DocLinkTargetIssue, nil
	case projectID != uuid.Nil:
		return DocLinkTargetProject, nil
	default:
		return "", NewValidationError(FieldError{Field: DocLinkFieldIssueID, Code: ValidationCodeRequired})
	}
}

type StoredDocLink struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	DocID              uuid.UUID
	IssueID            uuid.UUID
	ProjectID          uuid.UUID
	CreatedByAccountID uuid.UUID
	CreatedAt          time.Time
}

func (l StoredDocLink) Target() DocLinkTarget {
	if l.IssueID != uuid.Nil {
		return DocLinkTargetIssue
	}

	return DocLinkTargetProject
}

type DocLink struct {
	ID        uuid.UUID
	DocID     uuid.UUID
	Target    DocLinkTarget
	Issue     Issue
	Project   Project
	CreatedAt time.Time
}

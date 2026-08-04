package entity

import (
	"errors"
	"regexp"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	ProjectNameMinLen        = 1
	ProjectNameMaxLen        = 80
	ProjectSlugMinLen        = 2
	ProjectSlugMaxLen        = 48
	ProjectDescriptionMaxLen = 4000
	ProjectStatusBodyMinLen  = 1
	ProjectStatusBodyMaxLen  = 2000
)

var (
	ErrProjectNotFound           = errors.New("project not found")
	ErrProjectSlugTaken          = errors.New("project address already used in this workspace")
	ErrProjectArchived           = errors.New("project is archived")
	ErrProjectNotArchived        = errors.New("project is not archived")
	ErrProjectNotFinished        = errors.New("project has not been completed or cancelled")
	ErrProjectNotLead            = errors.New("only the project lead or an administrator may change this project")
	ErrProjectMemberExists       = errors.New("account is already on this project")
	ErrProjectMembershipNotFound = errors.New("project membership not found")
)

var projectSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type ProjectState string

const (
	ProjectStatePlanned   ProjectState = "planned"
	ProjectStateActive    ProjectState = "active"
	ProjectStatePaused    ProjectState = "paused"
	ProjectStateCompleted ProjectState = "completed"
	ProjectStateCancelled ProjectState = "cancelled"
)

func ProjectStates() []ProjectState {
	return []ProjectState{
		ProjectStatePlanned,
		ProjectStateActive,
		ProjectStatePaused,
		ProjectStateCompleted,
		ProjectStateCancelled,
	}
}

func (s ProjectState) Valid() bool {
	return slices.Contains(ProjectStates(), s)
}

func (s ProjectState) Finished() bool {
	return s == ProjectStateCompleted || s == ProjectStateCancelled
}

type ProjectHealth string

const (
	ProjectHealthOnTrack  ProjectHealth = "on_track"
	ProjectHealthAtRisk   ProjectHealth = "at_risk"
	ProjectHealthOffTrack ProjectHealth = "off_track"
)

func ProjectHealths() []ProjectHealth {
	return []ProjectHealth{ProjectHealthOnTrack, ProjectHealthAtRisk, ProjectHealthOffTrack}
}

func (h ProjectHealth) Valid() bool {
	return slices.Contains(ProjectHealths(), h)
}

type Project struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	Slug          string
	Name          string
	Description   string
	State         ProjectState
	LeadAccountID uuid.UUID
	LeadName      string
	TargetOn      string
	ArchivedAt    *time.Time
	Health        ProjectHealth
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (p Project) Archived() bool {
	return p.ArchivedAt != nil
}

func (p Project) LedBy(accountID uuid.UUID) bool {
	return p.LeadAccountID != uuid.Nil && p.LeadAccountID == accountID
}

type ProjectMembership struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	ProjectID   uuid.UUID
	AccountID   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProjectStatusUpdate struct {
	ID              uuid.UUID
	ProjectID       uuid.UUID
	AuthorAccountID uuid.UUID
	AuthorName      string
	Health          ProjectHealth
	Body            string
	CreatedAt       time.Time
}

func ValidateProjectName(field, name string) FieldError {
	switch length := utf8.RuneCountInString(name); {
	case length == 0:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length < ProjectNameMinLen:
		return FieldError{Field: field, Code: ValidationCodeTooShort}
	case length > ProjectNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateProjectSlug(field, slug string) FieldError {
	switch length := utf8.RuneCountInString(slug); {
	case length == 0:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length < ProjectSlugMinLen:
		return FieldError{Field: field, Code: ValidationCodeTooShort}
	case length > ProjectSlugMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !projectSlugPattern.MatchString(slug):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	default:
		return FieldError{}
	}
}

func ValidateProjectDescription(field, description string) FieldError {
	if utf8.RuneCountInString(description) > ProjectDescriptionMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateProjectStatusBody(field, body string) FieldError {
	switch length := utf8.RuneCountInString(body); {
	case length < ProjectStatusBodyMinLen:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > ProjectStatusBodyMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateTargetDate(field, date string) FieldError {
	if date == "" {
		return FieldError{}
	}

	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	return FieldError{}
}

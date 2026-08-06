package entity

import (
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	WorkspaceNameMinLen = 1
	WorkspaceNameMaxLen = 80
	WorkspaceSlugMinLen = 2
	WorkspaceSlugMaxLen = 40
)

var (
	ErrWorkspaceNotFound   = errors.New("workspace not found")
	ErrWorkspaceSlugTaken  = errors.New("workspace slug already taken")
	ErrWorkspaceDeleted    = errors.New("workspace is pending deletion")
	ErrWorkspaceNotDeleted = errors.New("workspace is not pending deletion")
)

type WorkspaceDeletedError struct {
	PurgeAfter *time.Time
}

func (e WorkspaceDeletedError) Error() string {
	if e.PurgeAfter == nil {
		return ErrWorkspaceDeleted.Error()
	}

	return fmt.Sprintf("%s until %s", ErrWorkspaceDeleted, e.PurgeAfter.Format(time.RFC3339))
}

func (e WorkspaceDeletedError) Unwrap() error {
	return ErrWorkspaceDeleted
}

var workspaceSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// A workspace is addressed at /<slug>, so a slug equal to a first path segment the edge already
// routes elsewhere would make that workspace permanently unreachable. These are refused as
// taken rather than as reserved: the caller gets the same answer, and the list stays private.
var reservedWorkspaceSlugs = map[string]struct{}{
	"v1":                {},
	"mcp":               {},
	"oauth":             {},
	"authorize":         {},
	"accept-invitation": {},
	"create-workspace":  {},
	"invite-teammates":  {},
	"reset-password":    {},
	"settings":          {},
	"sign-in":           {},
	"sign-up":           {},
	"sso":               {},
	"privacy":           {},
	"terms":             {},
}

func WorkspaceSlugReserved(slug string) bool {
	_, reserved := reservedWorkspaceSlugs[slug]

	return reserved
}

type WorkspaceStatus string

const (
	WorkspaceStatusActive          WorkspaceStatus = "active"
	WorkspaceStatusPendingDeletion WorkspaceStatus = "pending_deletion"
)

func (s WorkspaceStatus) Valid() bool {
	switch s {
	case WorkspaceStatusActive, WorkspaceStatusPendingDeletion:
		return true
	default:
		return false
	}
}

func (s WorkspaceStatus) CanTransitionTo(target WorkspaceStatus) bool {
	switch s {
	case WorkspaceStatusActive:
		return target == WorkspaceStatusPendingDeletion
	case WorkspaceStatusPendingDeletion:
		return target == WorkspaceStatusActive
	default:
		return false
	}
}

type Workspace struct {
	ID                  uuid.UUID
	Slug                string
	Name                string
	Status              WorkspaceStatus
	Timezone            string
	DefaultTeamID       *uuid.UUID
	DeletionRequestedAt *time.Time
	PurgeAfter          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (w Workspace) Deleted() bool {
	return w.Status == WorkspaceStatusPendingDeletion
}

func (w Workspace) PurgeDueAt(now time.Time) bool {
	return w.Deleted() && w.PurgeAfter != nil && !now.Before(*w.PurgeAfter)
}

func ValidateWorkspaceName(field, name string) FieldError {
	length := utf8.RuneCountInString(name)

	switch {
	case length < WorkspaceNameMinLen:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > WorkspaceNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateWorkspaceSlug(field, slug string) FieldError {
	length := utf8.RuneCountInString(slug)

	switch {
	case length == 0:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length < WorkspaceSlugMinLen:
		return FieldError{Field: field, Code: ValidationCodeTooShort}
	case length > WorkspaceSlugMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !workspaceSlugPattern.MatchString(slug):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	default:
		return FieldError{}
	}
}

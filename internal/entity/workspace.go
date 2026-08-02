package entity

import (
	"errors"
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
	ErrWorkspaceNotFound  = errors.New("workspace not found")
	ErrWorkspaceSlugTaken = errors.New("workspace slug already taken")
)

var workspaceSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Workspace struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
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

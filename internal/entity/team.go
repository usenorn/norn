package entity

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	TeamNameMinLen = 1
	TeamNameMaxLen = 80
	TeamKeyMinLen  = 2
	TeamKeyMaxLen  = 5
)

var (
	ErrTeamNotFound           = errors.New("team not found")
	ErrTeamKeyTaken           = errors.New("team key already taken")
	ErrTeamArchived           = errors.New("team is archived")
	ErrTeamNotArchived        = errors.New("team is not archived")
	ErrTeamMembershipNotFound = errors.New("team membership not found")
	ErrTeamMembershipExists   = errors.New("team membership already exists")
)

var teamKeyPattern = regexp.MustCompile(`^[A-Z]{2,5}$`)

type TeamStatus string

const (
	TeamStatusActive   TeamStatus = "active"
	TeamStatusArchived TeamStatus = "archived"
)

func (s TeamStatus) Valid() bool {
	switch s {
	case TeamStatusActive, TeamStatusArchived:
		return true
	default:
		return false
	}
}

func (s TeamStatus) CanTransitionTo(target TeamStatus) bool {
	switch s {
	case TeamStatusActive:
		return target == TeamStatusArchived
	case TeamStatusArchived:
		return target == TeamStatusActive
	default:
		return false
	}
}

type TeamVisibility string

const (
	TeamVisibilityPublic  TeamVisibility = "public"
	TeamVisibilityPrivate TeamVisibility = "private"
)

const DefaultTeamVisibility = TeamVisibilityPublic

func (v TeamVisibility) Valid() bool {
	switch v {
	case TeamVisibilityPublic, TeamVisibilityPrivate:
		return true
	default:
		return false
	}
}

type Team struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Key         string
	Name        string
	Status      TeamStatus
	Visibility  TeamVisibility
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (t Team) Archived() bool {
	return t.Status == TeamStatusArchived
}

func (t Team) Private() bool {
	return t.Visibility == TeamVisibilityPrivate
}

type TeamMembership struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	TeamID      uuid.UUID
	AccountID   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NormalizeTeamKey(key string) string {
	return strings.ToUpper(strings.TrimSpace(key))
}

func ValidateTeamName(field, name string) FieldError {
	length := utf8.RuneCountInString(strings.TrimSpace(name))

	switch {
	case length < TeamNameMinLen:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > TeamNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateTeamKey(field, key string) FieldError {
	length := utf8.RuneCountInString(key)

	switch {
	case length == 0:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length < TeamKeyMinLen:
		return FieldError{Field: field, Code: ValidationCodeTooShort}
	case length > TeamKeyMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !teamKeyPattern.MatchString(key):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	default:
		return FieldError{}
	}
}

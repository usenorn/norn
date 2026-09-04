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
	TeamNameMinLen        = 1
	TeamNameMaxLen        = 80
	TeamKeyMinLen         = 2
	TeamKeyMaxLen         = 5
	TeamDescriptionMaxLen = 500
	TeamIconMaxLen        = 40
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

type TeamColor string

const DefaultTeamColor = TeamColor(LabelColorNeutral)

func (c TeamColor) Valid() bool {
	return LabelColor(c).Valid()
}

func TeamColors() []TeamColor {
	palette := LabelColors()
	colors := make([]TeamColor, 0, len(palette))

	for _, color := range palette {
		colors = append(colors, TeamColor(color))
	}

	return colors
}

type TeamEstimation string

const (
	TeamEstimationNone   TeamEstimation = "none"
	TeamEstimationPoints TeamEstimation = "points"
	TeamEstimationHours  TeamEstimation = "hours"
	TeamEstimationSizes  TeamEstimation = "sizes"
)

const DefaultTeamEstimation = TeamEstimationNone

func (e TeamEstimation) Valid() bool {
	switch e {
	case TeamEstimationNone, TeamEstimationPoints, TeamEstimationHours, TeamEstimationSizes:
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
	Description string
	Icon        string
	IconColor   TeamColor
	Estimation  TeamEstimation
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

func ValidateTeamDescription(field, description string) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(description)) > TeamDescriptionMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateTeamIcon(field, icon string) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(icon)) > TeamIconMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateTeamColor(field string, color TeamColor) FieldError {
	if !color.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func ValidateTeamEstimation(field string, estimation TeamEstimation) FieldError {
	if !estimation.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

package entity

import (
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	SavedViewNameMinLen = 1
	SavedViewNameMaxLen = 80
)

var (
	ErrSavedViewNotFound     = errors.New("saved view not found")
	ErrSavedViewNotOwner     = errors.New("only the person who saved this view or an administrator may change it")
	ErrSavedViewNotShareable = errors.New("a viewer may keep personal views but may not share one")
	ErrSavedViewShared       = errors.New("this view is shared and its removal was not acknowledged")
)

type SavedViewSharing string

const (
	SavedViewSharingPersonal  SavedViewSharing = "personal"
	SavedViewSharingTeam      SavedViewSharing = "team"
	SavedViewSharingWorkspace SavedViewSharing = "workspace"
)

func SavedViewSharings() []SavedViewSharing {
	return []SavedViewSharing{
		SavedViewSharingPersonal,
		SavedViewSharingTeam,
		SavedViewSharingWorkspace,
	}
}

func (s SavedViewSharing) Valid() bool {
	return slices.Contains(SavedViewSharings(), s)
}

func (s SavedViewSharing) Shared() bool {
	return s == SavedViewSharingTeam || s == SavedViewSharingWorkspace
}

type SavedView struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	AuthorID    uuid.UUID
	AuthorName  string
	Sharing     SavedViewSharing
	TeamID      uuid.UUID
	TeamName    string
	Name        string
	Filter      IssueFilter
	Sort        []IssueSort
	GroupBy     IssueGroupBy
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (v SavedView) Shared() bool {
	return v.Sharing.Shared()
}

func (v SavedView) AuthoredBy(accountID uuid.UUID) bool {
	return v.AuthorID != uuid.Nil && v.AuthorID == accountID
}

func (v SavedView) VisibleTo(accountID uuid.UUID, teamIDs []uuid.UUID) bool {
	switch v.Sharing {
	case SavedViewSharingWorkspace:
		return true
	case SavedViewSharingTeam:
		return slices.Contains(teamIDs, v.TeamID)
	default:
		return v.AuthoredBy(accountID)
	}
}

func (v SavedView) EditableBy(accountID uuid.UUID, role MembershipRole) bool {
	return v.AuthoredBy(accountID) || (v.Shared() && role == MembershipRoleAdmin)
}

func (v SavedView) Validate() error {
	if err := NewValidationError(ValidateSavedViewName("name", v.Name)); err != nil {
		return err
	}

	if !v.Sharing.Valid() {
		return NewValidationError(FieldError{Field: "sharing", Code: ValidationCodeUnsupportedValue})
	}

	if (v.Sharing == SavedViewSharingTeam) != (v.TeamID != uuid.Nil) {
		return NewValidationError(FieldError{Field: "teamId", Code: ValidationCodeRequired})
	}

	if v.GroupBy != "" && !v.GroupBy.Valid() {
		return ErrIssueGroupUnknown
	}

	if err := v.Filter.Validate(); err != nil {
		return err
	}

	_, err := NormalizedIssueSort(v.Sort)

	return err
}

func ValidateSavedViewName(field, name string) FieldError {
	switch length := utf8.RuneCountInString(name); {
	case length == 0:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length < SavedViewNameMinLen:
		return FieldError{Field: field, Code: ValidationCodeTooShort}
	case length > SavedViewNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

type SavedViewSharedError struct {
	Sharing  SavedViewSharing
	TeamID   uuid.UUID
	TeamName string
}

func (e SavedViewSharedError) Error() string {
	if e.Sharing == SavedViewSharingTeam && e.TeamName != "" {
		return fmt.Sprintf("%s: team %s", ErrSavedViewShared, e.TeamName)
	}

	return fmt.Sprintf("%s: %s", ErrSavedViewShared, e.Sharing)
}

func (e SavedViewSharedError) Unwrap() error {
	return ErrSavedViewShared
}

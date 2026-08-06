package entity

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	LabelNameMinLen      = 1
	LabelNameMaxLen      = 40
	LabelGroupNameMinLen = 1
	LabelGroupNameMaxLen = 40
)

var (
	ErrLabelNotFound           = errors.New("label not found")
	ErrLabelNameTaken          = errors.New("label name already used in this scope")
	ErrLabelOutOfScope         = errors.New("label does not apply to this issue's team")
	ErrLabelUsageChanged       = errors.New("label is on more issues than were acknowledged")
	ErrLabelMergeScopeNarrows  = errors.New("merge target does not cover the source label's scope")
	ErrLabelMergeGroupMismatch = errors.New("merge is only possible between labels in the same group")
	ErrLabelGroupNotFound      = errors.New("label group not found")
	ErrLabelGroupNameTaken     = errors.New("label group name already used in this workspace")
	ErrLabelGroupInUse         = errors.New("label group still has labels in it")
	ErrLabelGroupExclusive     = errors.New("an issue may carry only one label from a group")
)

type LabelColor string

const (
	LabelColorNeutral LabelColor = "neutral"
	LabelColorCyan    LabelColor = "cyan"
	LabelColorBlue    LabelColor = "blue"
	LabelColorViolet  LabelColor = "violet"
	LabelColorOrchid  LabelColor = "orchid"
	LabelColorMagenta LabelColor = "magenta"
)

func (c LabelColor) Valid() bool {
	switch c {
	case LabelColorNeutral, LabelColorCyan, LabelColorBlue,
		LabelColorViolet, LabelColorOrchid, LabelColorMagenta:
		return true
	default:
		return false
	}
}

func LabelColors() []LabelColor {
	return []LabelColor{
		LabelColorNeutral,
		LabelColorCyan,
		LabelColorBlue,
		LabelColorViolet,
		LabelColorOrchid,
		LabelColorMagenta,
	}
}

type Label struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	TeamID      uuid.UUID
	GroupID     uuid.UUID
	Name        string
	Color       LabelColor
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Origin      *ImportOrigin
}

func (l Label) AppliesTo(teamID uuid.UUID) bool {
	return l.TeamID == uuid.Nil || l.TeamID == teamID
}

func (l Label) Covers(other Label) bool {
	if l.WorkspaceID != other.WorkspaceID {
		return false
	}

	return l.TeamID == uuid.Nil || l.TeamID == other.TeamID
}

type LabelGroup struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type LabelUsage struct {
	Issues int
}

type LabelUsageChangedError struct {
	Issues int
}

func (e LabelUsageChangedError) Error() string {
	return fmt.Sprintf("%s: %d", ErrLabelUsageChanged, e.Issues)
}

func (e LabelUsageChangedError) Unwrap() error {
	return ErrLabelUsageChanged
}

func ValidateLabelName(field, name string) FieldError {
	length := utf8.RuneCountInString(strings.TrimSpace(name))

	switch {
	case length < LabelNameMinLen:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > LabelNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateLabelGroupName(field, name string) FieldError {
	length := utf8.RuneCountInString(strings.TrimSpace(name))

	switch {
	case length < LabelGroupNameMinLen:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > LabelGroupNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateLabelColor(field string, color LabelColor) FieldError {
	if !color.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func GroupedLabelConflict(labels []Label) bool {
	seen := make(map[uuid.UUID]struct{}, len(labels))

	for _, label := range labels {
		if label.GroupID == uuid.Nil {
			continue
		}

		if _, clash := seen[label.GroupID]; clash {
			return true
		}

		seen[label.GroupID] = struct{}{}
	}

	return false
}

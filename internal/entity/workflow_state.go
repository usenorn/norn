package entity

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	WorkflowStateNameMinLen = 1
	WorkflowStateNameMaxLen = 60
)

var (
	ErrWorkflowStateNotFound       = errors.New("workflow state not found")
	ErrWorkflowStateNameTaken      = errors.New("workflow state name already used in this team")
	ErrWorkflowStateLastInCategory = errors.New("workflow state is the last one in its category")
	ErrWorkflowStateIsDefault      = errors.New("workflow state is the team default for new issues")
	ErrWorkflowStateIsCompletion   = errors.New("workflow state is the team completion state")
)

type StateCategory string

const (
	StateCategoryNotStarted StateCategory = "not_started"
	StateCategoryActive     StateCategory = "active"
	StateCategoryComplete   StateCategory = "complete"
	StateCategoryAbandoned  StateCategory = "abandoned"
)

func (c StateCategory) Valid() bool {
	switch c {
	case StateCategoryNotStarted, StateCategoryActive, StateCategoryComplete, StateCategoryAbandoned:
		return true
	default:
		return false
	}
}

func StateCategories() []StateCategory {
	return []StateCategory{
		StateCategoryNotStarted,
		StateCategoryActive,
		StateCategoryComplete,
		StateCategoryAbandoned,
	}
}

type WorkflowState struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	TeamID       uuid.UUID
	Name         string
	Category     StateCategory
	Position     int
	IsDefault    bool
	IsCompletion bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Origin       *ImportOrigin
}

func ValidateWorkflowStateName(field, name string) FieldError {
	length := utf8.RuneCountInString(strings.TrimSpace(name))

	switch {
	case length < WorkflowStateNameMinLen:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > WorkflowStateNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func DefaultWorkflowStates(workspaceID, teamID uuid.UUID) []WorkflowState {
	seed := []struct {
		name       string
		category   StateCategory
		isDefault  bool
		completion bool
	}{
		{"Backlog", StateCategoryNotStarted, false, false},
		{"Todo", StateCategoryNotStarted, true, false},
		{"In progress", StateCategoryActive, false, false},
		{"In review", StateCategoryActive, false, false},
		{"Done", StateCategoryComplete, false, true},
		{"Canceled", StateCategoryAbandoned, false, false},
	}

	states := make([]WorkflowState, 0, len(seed))

	for i, entry := range seed {
		states = append(states, WorkflowState{
			WorkspaceID:  workspaceID,
			TeamID:       teamID,
			Name:         entry.name,
			Category:     entry.category,
			Position:     i + 1,
			IsDefault:    entry.isDefault,
			IsCompletion: entry.completion,
		})
	}

	return states
}

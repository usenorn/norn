package entity

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrIssueAlreadyOnTeam        = errors.New("issue is already on that team")
	ErrIssueLabelsOutOfScope     = errors.New("issue carries labels the destination team cannot hold")
	ErrIssueDestinationIncapable = errors.New("destination team has no state in the category the issue is in")
)

type IssueLabelsOutOfScopeError struct {
	Labels []Label
}

func (e IssueLabelsOutOfScopeError) Error() string {
	names := make([]string, 0, len(e.Labels))

	for _, label := range e.Labels {
		names = append(names, label.Name)
	}

	return fmt.Sprintf("%s: %s", ErrIssueLabelsOutOfScope, strings.Join(names, ", "))
}

func (e IssueLabelsOutOfScopeError) Unwrap() error {
	return ErrIssueLabelsOutOfScope
}

func CounterpartState(states []WorkflowState, category StateCategory) (WorkflowState, bool) {
	var counterpart WorkflowState

	for _, state := range states {
		if state.Category != category {
			continue
		}

		if counterpart.ID == uuid.Nil || state.Position < counterpart.Position {
			counterpart = state
		}
	}

	return counterpart, counterpart.ID != uuid.Nil
}

func LabelsOutOfScope(labels []Label, destination uuid.UUID) []Label {
	stranded := make([]Label, 0, len(labels))

	for _, label := range labels {
		if label.TeamID != uuid.Nil && label.TeamID != destination {
			stranded = append(stranded, label)
		}
	}

	return stranded
}

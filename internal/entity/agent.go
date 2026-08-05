package entity

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	AgentNameMaxLen = 80

	AgentActionWindow     = time.Minute
	AgentActionsPerWindow = 120
)

var (
	ErrAgentNotFound     = errors.New("agent not found")
	ErrAgentNameTaken    = errors.New("agent name already used in this workspace")
	ErrAgentDisabled     = errors.New("agent is disabled")
	ErrAgentRateLimited  = errors.New("agent is acting faster than its allowance")
	ErrAgentOwnerInvalid = errors.New("agent owner is not a member of this workspace")
)

type AgentStatus string

const (
	AgentStatusActive   AgentStatus = "active"
	AgentStatusDisabled AgentStatus = "disabled"
)

func AgentStatuses() []AgentStatus {
	return []AgentStatus{AgentStatusActive, AgentStatusDisabled}
}

func (s AgentStatus) Valid() bool {
	return slices.Contains(AgentStatuses(), s)
}

func (s AgentStatus) CanTransitionTo(target AgentStatus) bool {
	if !s.Valid() || !target.Valid() || s == target {
		return false
	}

	return true
}

type Agent struct {
	ID             uuid.UUID
	WorkspaceID    uuid.UUID
	AccountID      uuid.UUID
	OwnerAccountID uuid.UUID
	Name           string
	Status         AgentStatus
	ActionLimit    *int
	DisabledAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (a Agent) Disabled() bool {
	return a.Status == AgentStatusDisabled
}

func (a Agent) Allowance() int {
	if a.ActionLimit == nil || *a.ActionLimit <= 0 {
		return AgentActionsPerWindow
	}

	return *a.ActionLimit
}

func ValidateAgentName(field, name string) FieldError {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > AgentNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

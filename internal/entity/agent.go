package entity

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	AgentNameMaxLen     = 80
	AgentActionLimitMin = 1
	AgentActionLimitMax = 6000
	DefaultAgentIcon    = AgentIconBot

	AgentActionWindow     = time.Minute
	AgentActionsPerWindow = 120
)

var (
	ErrAgentNotFound         = errors.New("agent not found")
	ErrAgentNameTaken        = errors.New("agent name already used in this workspace")
	ErrAgentDisabled         = errors.New("agent is disabled")
	ErrAgentActive           = errors.New("agent is active")
	ErrAgentAuthorityMissing = errors.New("agent authority cannot be restored")
	ErrAgentRateLimited      = errors.New("agent is acting faster than its allowance")
	ErrAgentOwnerInvalid     = errors.New("agent owner must be an active person in this workspace")
)

type AgentIcon string

const (
	AgentIconBot            AgentIcon = "bot"
	AgentIconInbox          AgentIcon = "inbox"
	AgentIconSearch         AgentIcon = "search"
	AgentIconTerminal       AgentIcon = "terminal"
	AgentIconPencil         AgentIcon = "pencil"
	AgentIconGitPullRequest AgentIcon = "git-pull-request"
	AgentIconShieldCheck    AgentIcon = "shield-check"
	AgentIconScrollText     AgentIcon = "scroll-text"
	AgentIconTarget         AgentIcon = "target"
	AgentIconSparkles       AgentIcon = "sparkles"
)

func (i AgentIcon) Valid() bool {
	switch i {
	case AgentIconBot,
		AgentIconInbox,
		AgentIconSearch,
		AgentIconTerminal,
		AgentIconPencil,
		AgentIconGitPullRequest,
		AgentIconShieldCheck,
		AgentIconScrollText,
		AgentIconTarget,
		AgentIconSparkles:
		return true
	default:
		return false
	}
}

func (i AgentIcon) Normalized() AgentIcon {
	if i == "" {
		return DefaultAgentIcon
	}

	return i
}

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
	Icon           AgentIcon
	Status         AgentStatus
	ActionLimit    *int
	DisabledAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (a Agent) Disabled() bool {
	return a.Status == AgentStatusDisabled
}

func (a Agent) OwnedBy(accountID uuid.UUID) bool {
	return a.OwnerAccountID != uuid.Nil && a.OwnerAccountID == accountID
}

func (a Agent) ManageableBy(accountID uuid.UUID, role MembershipRole) bool {
	return a.OwnedBy(accountID) || role == MembershipRoleAdmin
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

func ValidateAgentIcon(field string, icon AgentIcon) FieldError {
	if !icon.Normalized().Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func ValidateAgentActionLimit(field string, limit *int) FieldError {
	if limit != nil && (*limit < AgentActionLimitMin || *limit > AgentActionLimitMax) {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	return FieldError{}
}

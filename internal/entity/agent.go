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

	DefaultAgentScope = AgentScopeMember
)

var (
	ErrAgentNotFound         = errors.New("agent not found")
	ErrAgentNameTaken        = errors.New("agent name already used in this workspace")
	ErrAgentDisabled         = errors.New("agent is disabled")
	ErrAgentActive           = errors.New("agent is active")
	ErrAgentAuthorityMissing = errors.New("agent authority cannot be restored")
	ErrAgentRateLimited      = errors.New("agent is acting faster than its allowance")
	ErrAgentOwnerInvalid     = errors.New("agent owner must be an active person in this workspace")
	ErrAgentScopeForbidden   = errors.New("that agent scope is not yours to grant")
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

type AgentScope string

const (
	AgentScopeMember    AgentScope = "member"
	AgentScopeProject   AgentScope = "project"
	AgentScopeWorkspace AgentScope = "workspace"
)

func AgentScopes() []AgentScope {
	return []AgentScope{AgentScopeMember, AgentScopeProject, AgentScopeWorkspace}
}

func (s AgentScope) Valid() bool {
	return slices.Contains(AgentScopes(), s)
}

func (s AgentScope) Normalized() AgentScope {
	if s == "" {
		return DefaultAgentScope
	}

	return s
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
	ID                uuid.UUID
	WorkspaceID       uuid.UUID
	AccountID         uuid.UUID
	OwnerAccountID    uuid.UUID
	Name              string
	Icon              AgentIcon
	Status            AgentStatus
	Scope             AgentScope
	ProjectID         *uuid.UUID
	ActionLimit       *int
	AgentInstructions string
	DisabledAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
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

type AgentDelegation struct {
	AccountID      uuid.UUID
	IssueProjectID uuid.UUID
	InProject      bool
}

func (a Agent) DelegatableBy(request AgentDelegation) bool {
	switch a.Scope.Normalized() {
	case AgentScopeWorkspace:
		return true
	case AgentScopeProject:
		if a.ProjectID == nil || *a.ProjectID != request.IssueProjectID {
			return false
		}

		return request.InProject || a.OwnedBy(request.AccountID)
	default:
		return a.OwnedBy(request.AccountID)
	}
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

func ValidateAgentScope(scopeField, projectField string, scope AgentScope, projectID *uuid.UUID) []FieldError {
	normalized := scope.Normalized()

	if !normalized.Valid() {
		return []FieldError{{Field: scopeField, Code: ValidationCodeUnsupportedValue}}
	}

	switch {
	case normalized == AgentScopeProject && projectID == nil:
		return []FieldError{{Field: projectField, Code: ValidationCodeRequired}}
	case normalized != AgentScopeProject && projectID != nil:
		return []FieldError{{Field: projectField, Code: ValidationCodeUnsupportedValue}}
	default:
		return nil
	}
}

func ValidateAgentActionLimit(field string, limit *int) FieldError {
	if limit != nil && (*limit < AgentActionLimitMin || *limit > AgentActionLimitMax) {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	return FieldError{}
}

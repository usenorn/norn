package entity

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAgentActionHeld       = errors.New("agent action is waiting for a person to approve it")
	ErrAgentProposalNotFound = errors.New("agent proposal not found")
	ErrAgentProposalSettled  = errors.New("agent proposal has already been decided")
)

type AgentAction string

const (
	AgentActionComment     AgentAction = "comment"
	AgentActionStateChange AgentAction = "state_change"
	AgentActionIssueEdit   AgentAction = "issue_edit"
)

func AgentActions() []AgentAction {
	return []AgentAction{AgentActionComment, AgentActionStateChange, AgentActionIssueEdit}
}

func (a AgentAction) Valid() bool {
	return slices.Contains(AgentActions(), a)
}

type AgentSettings struct {
	TeamID           uuid.UUID
	WorkspaceID      uuid.UUID
	HoldComments     bool
	HoldStateChanges bool
	HoldIssueEdits   bool
}

func (s AgentSettings) Holds(action AgentAction) bool {
	switch action {
	case AgentActionComment:
		return s.HoldComments
	case AgentActionStateChange:
		return s.HoldStateChanges
	case AgentActionIssueEdit:
		return s.HoldIssueEdits
	default:
		return false
	}
}

func (a AgentAction) Scopes() APIScopeSet {
	switch a {
	case AgentActionComment:
		return APIScopeSet{NewAPIScope(ResourceComment, ActionManage)}
	case AgentActionStateChange, AgentActionIssueEdit:
		return APIScopeSet{NewAPIScope(ResourceIssue, ActionManage)}
	default:
		return APIScopeSet{}
	}
}

func (s AgentSettings) HoldsAnything() bool {
	return s.HoldComments || s.HoldStateChanges || s.HoldIssueEdits
}

type AgentActionHeldError struct {
	ProposalID uuid.UUID
}

func (e AgentActionHeldError) Error() string {
	return ErrAgentActionHeld.Error()
}

func (e AgentActionHeldError) Unwrap() error {
	return ErrAgentActionHeld
}

type AgentProposalStatus string

const (
	AgentProposalPending  AgentProposalStatus = "pending"
	AgentProposalRejected AgentProposalStatus = "rejected"
	AgentProposalApplied  AgentProposalStatus = "applied"
	AgentProposalFailed   AgentProposalStatus = "failed"
)

func AgentProposalStatuses() []AgentProposalStatus {
	return []AgentProposalStatus{
		AgentProposalPending,
		AgentProposalRejected,
		AgentProposalApplied,
		AgentProposalFailed,
	}
}

func (s AgentProposalStatus) Valid() bool {
	return slices.Contains(AgentProposalStatuses(), s)
}

func (s AgentProposalStatus) Settled() bool {
	return s.Valid() && s != AgentProposalPending
}

func (s AgentProposalStatus) CanTransitionTo(target AgentProposalStatus) bool {
	if !s.Valid() || !target.Valid() {
		return false
	}

	return s == AgentProposalPending && target != AgentProposalPending
}

type AgentChange struct {
	ExpectedVersion int
	Body            string
	StateID         *uuid.UUID
	Title           *string
	Priority        *IssuePriority
	AssigneeID      *uuid.UUID
}

type AgentProposal struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	AgentID     uuid.UUID
	AgentName   string
	IssueID     uuid.UUID
	TeamID      uuid.UUID
	Action      AgentAction
	Change      AgentChange
	Status      AgentProposalStatus
	DecidedBy   uuid.UUID
	DecidedAt   *time.Time
	Failure     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

package entity

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrAgentActionHeld          = errors.New("agent action is waiting for a person to approve it")
	ErrAgentProposalNotFound    = errors.New("agent proposal not found")
	ErrAgentProposalSettled     = errors.New("agent proposal has already been decided")
	ErrAgentProposalNotEditable = errors.New("only a proposed check set can be edited while approving")
)

type AgentAction string

const (
	AgentActionComment     AgentAction = "comment"
	AgentActionStateChange AgentAction = "state_change"
	AgentActionIssueEdit   AgentAction = "issue_edit"
	AgentActionIssueCreate AgentAction = "issue_create"
	AgentActionCheckSet    AgentAction = "check_set"
)

func AgentActions() []AgentAction {
	return []AgentAction{
		AgentActionComment,
		AgentActionStateChange,
		AgentActionIssueEdit,
		AgentActionIssueCreate,
		AgentActionCheckSet,
	}
}

func (a AgentAction) Valid() bool {
	return slices.Contains(AgentActions(), a)
}

func (a AgentAction) JudgedOnProof() bool {
	return a == AgentActionStateChange
}

type AgentHold string

const (
	AgentHoldNever        AgentHold = "never"
	AgentHoldUnlessProven AgentHold = "unless_proven"
	AgentHoldAlways       AgentHold = "always"
)

var agentHoldOrder = []AgentHold{AgentHoldNever, AgentHoldUnlessProven, AgentHoldAlways}

func AgentHolds() []AgentHold {
	return slices.Clone(agentHoldOrder)
}

func (h AgentHold) Valid() bool {
	return slices.Contains(agentHoldOrder, h)
}

func (h AgentHold) Stronger(other AgentHold) bool {
	return slices.Index(agentHoldOrder, h) > slices.Index(agentHoldOrder, other)
}

func ValidateAgentHold(field string, hold AgentHold, action AgentAction) FieldError {
	if !hold.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	if hold == AgentHoldUnlessProven && !action.JudgedOnProof() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

type AgentSettings struct {
	TeamID            uuid.UUID
	WorkspaceID       uuid.UUID
	HoldComments      AgentHold
	HoldStateChanges  AgentHold
	HoldIssueEdits    AgentHold
	HoldIssueCreation AgentHold
}

func (s AgentSettings) Holds(action AgentAction) AgentHold {
	switch action {
	case AgentActionComment:
		return normalisedHold(s.HoldComments)
	case AgentActionStateChange:
		return normalisedHold(s.HoldStateChanges)
	case AgentActionIssueEdit:
		return normalisedHold(s.HoldIssueEdits)
	case AgentActionIssueCreate:
		return normalisedHold(s.HoldIssueCreation)
	case AgentActionCheckSet:
		return AgentHoldAlways
	default:
		return AgentHoldNever
	}
}

func (s AgentSettings) Strongest(actions []AgentAction) (AgentAction, AgentHold) {
	decided, strongest := AgentAction(""), AgentHoldNever

	for _, action := range actions {
		if hold := s.Holds(action); decided == "" || hold.Stronger(strongest) {
			decided, strongest = action, hold
		}
	}

	return decided, strongest
}

func (s AgentSettings) Normalised() AgentSettings {
	s.HoldComments = normalisedHold(s.HoldComments)
	s.HoldStateChanges = normalisedHold(s.HoldStateChanges)
	s.HoldIssueEdits = normalisedHold(s.HoldIssueEdits)
	s.HoldIssueCreation = normalisedHold(s.HoldIssueCreation)

	return s
}

func normalisedHold(hold AgentHold) AgentHold {
	if !hold.Valid() {
		return AgentHoldNever
	}

	return hold
}

func (a AgentAction) Scopes() APIScopeSet {
	switch a {
	case AgentActionComment:
		return APIScopeSet{NewAPIScope(ResourceComment, ActionManage)}
	case AgentActionStateChange, AgentActionIssueEdit, AgentActionIssueCreate:
		return APIScopeSet{NewAPIScope(ResourceIssue, ActionManage)}
	case AgentActionCheckSet:
		return APIScopeSet{NewAPIScope(ResourceCheck, ActionManage)}
	default:
		return APIScopeSet{}
	}
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
	ExpectedVersion int            `json:"expectedVersion,omitempty"`
	Body            string         `json:"body,omitempty"`
	StateID         *uuid.UUID     `json:"stateId,omitempty"`
	Title           *string        `json:"title,omitempty"`
	Description     *string        `json:"description,omitempty"`
	Priority        *IssuePriority `json:"priority,omitempty"`
	AssigneeID      *uuid.UUID     `json:"assigneeId,omitempty"`
	Estimate        *int           `json:"estimate,omitempty"`
	DueOn           *string        `json:"dueOn,omitempty"`
	CycleID         *uuid.UUID     `json:"cycleId,omitempty"`
	ProjectID       *uuid.UUID     `json:"projectId,omitempty"`
	Clear           []string       `json:"clear,omitempty"`
	CheckIDs        []uuid.UUID    `json:"checkIds,omitempty"`
	LabelIDs        []uuid.UUID    `json:"labelIds,omitempty"`
}

const (
	AgentReasoningMaxLen     = 4000
	AgentSourcesMax          = 20
	AgentSourceLabelMaxLen   = 200
	AgentSourceAddressMaxLen = 2048
)

type AgentSource struct {
	Label string `json:"label"`
	URL   string `json:"url,omitempty"`
}

type AgentReasoning struct {
	Observed  string        `json:"observed,omitempty"`
	Consulted []AgentSource `json:"consulted,omitempty"`
	Uncertain string        `json:"uncertain,omitempty"`
}

func (r AgentReasoning) Empty() bool {
	return r.Observed == "" && r.Uncertain == "" && len(r.Consulted) == 0
}

func ValidateAgentReasoning(field string, reasoning AgentReasoning) FieldError {
	if utf8.RuneCountInString(reasoning.Observed) > AgentReasoningMaxLen ||
		utf8.RuneCountInString(reasoning.Uncertain) > AgentReasoningMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	if len(reasoning.Consulted) > AgentSourcesMax {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	for _, source := range reasoning.Consulted {
		if strings.TrimSpace(source.Label) == "" {
			return FieldError{Field: field, Code: ValidationCodeRequired}
		}

		if utf8.RuneCountInString(source.Label) > AgentSourceLabelMaxLen ||
			utf8.RuneCountInString(source.URL) > AgentSourceAddressMaxLen {
			return FieldError{Field: field, Code: ValidationCodeTooLong}
		}
	}

	return FieldError{}
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
	Reasoning   AgentReasoning
	Status      AgentProposalStatus
	DecidedBy   uuid.UUID
	DecidedAt   *time.Time
	Failure     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

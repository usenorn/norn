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
	ErrAgentActionHeld       = errors.New("agent action is waiting for a person to approve it")
	ErrAgentProposalNotFound = errors.New("agent proposal not found")
	ErrAgentProposalSettled  = errors.New("agent proposal has already been decided")
)

type AgentAction string

const (
	AgentActionComment     AgentAction = "comment"
	AgentActionStateChange AgentAction = "state_change"
	AgentActionIssueEdit   AgentAction = "issue_edit"
	AgentActionIssueCreate AgentAction = "issue_create"
)

func AgentActions() []AgentAction {
	return []AgentAction{
		AgentActionComment,
		AgentActionStateChange,
		AgentActionIssueEdit,
		AgentActionIssueCreate,
	}
}

func (a AgentAction) Valid() bool {
	return slices.Contains(AgentActions(), a)
}

type AgentHold string

const (
	AgentHoldNever  AgentHold = "never"
	AgentHoldAlways AgentHold = "always"
)

var agentHoldOrder = []AgentHold{AgentHoldNever, AgentHoldAlways}

func AgentHolds() []AgentHold {
	return slices.Clone(agentHoldOrder)
}

func (h AgentHold) Valid() bool {
	return slices.Contains(agentHoldOrder, h)
}

func (h AgentHold) Stronger(other AgentHold) bool {
	return slices.Index(agentHoldOrder, h) > slices.Index(agentHoldOrder, other)
}

func ValidateAgentHold(field string, hold AgentHold) FieldError {
	if !hold.Valid() {
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

// ChangePart names one thing a proposal asks to change. An approver decides part by part, so a
// good title and a description that misses the point are not one decision.
type ChangePart string

const (
	ChangePartTitle       ChangePart = "title"
	ChangePartDescription ChangePart = "description"
	ChangePartState       ChangePart = "state"
	ChangePartPriority    ChangePart = "priority"
	ChangePartAssignee    ChangePart = "assignee"
	ChangePartEstimate    ChangePart = "estimate"
	ChangePartDueOn       ChangePart = "dueOn"
	ChangePartCycle       ChangePart = "cycle"
	ChangePartProject     ChangePart = "project"
	ChangePartLabels      ChangePart = "labels"
)

func ChangeParts() []ChangePart {
	return []ChangePart{
		ChangePartTitle, ChangePartDescription, ChangePartState, ChangePartPriority,
		ChangePartAssignee, ChangePartEstimate, ChangePartDueOn, ChangePartCycle,
		ChangePartProject, ChangePartLabels,
	}
}

func (p ChangePart) Valid() bool {
	return slices.Contains(ChangeParts(), p)
}

// Parts reads what a change actually asks for, so a reader is shown the four things it proposes
// rather than the ten it could have.
func (c AgentChange) Parts() []ChangePart {
	var asked []ChangePart

	for part, proposed := range map[ChangePart]bool{
		ChangePartTitle:       c.Title != nil,
		ChangePartDescription: c.Description != nil,
		ChangePartState:       c.StateID != nil,
		ChangePartPriority:    c.Priority != nil,
		ChangePartAssignee:    c.AssigneeID != nil || slices.Contains(c.Clear, "assignee"),
		ChangePartEstimate:    c.Estimate != nil || slices.Contains(c.Clear, "estimate"),
		ChangePartDueOn:       c.DueOn != nil || slices.Contains(c.Clear, "dueOn"),
		ChangePartCycle:       c.CycleID != nil || slices.Contains(c.Clear, "cycle"),
		ChangePartProject:     c.ProjectID != nil || slices.Contains(c.Clear, "project"),
		ChangePartLabels:      len(c.LabelIDs) > 0,
	} {
		if proposed {
			asked = append(asked, part)
		}
	}

	slices.SortFunc(asked, func(one, other ChangePart) int {
		return slices.Index(ChangeParts(), one) - slices.Index(ChangeParts(), other)
	})

	return asked
}

// Only narrows a change to the parts an approver accepted. Naming nothing accepts the whole
// change, which is what an approver who did not choose meant.
func (c AgentChange) Only(accepted []ChangePart) AgentChange {
	if len(accepted) == 0 {
		return c
	}

	taken := func(part ChangePart) bool { return slices.Contains(accepted, part) }

	narrowed := AgentChange{ExpectedVersion: c.ExpectedVersion, Body: c.Body}

	if taken(ChangePartTitle) {
		narrowed.Title = c.Title
	}

	if taken(ChangePartDescription) {
		narrowed.Description = c.Description
	}

	if taken(ChangePartState) {
		narrowed.StateID = c.StateID
	}

	if taken(ChangePartPriority) {
		narrowed.Priority = c.Priority
	}

	if taken(ChangePartAssignee) {
		narrowed.AssigneeID = c.AssigneeID
	}

	if taken(ChangePartEstimate) {
		narrowed.Estimate = c.Estimate
	}

	if taken(ChangePartDueOn) {
		narrowed.DueOn = c.DueOn
	}

	if taken(ChangePartCycle) {
		narrowed.CycleID = c.CycleID
	}

	if taken(ChangePartProject) {
		narrowed.ProjectID = c.ProjectID
	}

	if taken(ChangePartLabels) {
		narrowed.LabelIDs = c.LabelIDs
	}

	for _, cleared := range c.Clear {
		if taken(ChangePart(cleared)) {
			narrowed.Clear = append(narrowed.Clear, cleared)
		}
	}

	return narrowed
}

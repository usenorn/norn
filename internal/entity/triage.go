package entity

import (
	"errors"
	"slices"

	"github.com/google/uuid"
)

var (
	ErrTriageDisabled  = errors.New("team does not triage incoming issues")
	ErrIssueNotWaiting = errors.New("issue is not waiting in triage")
)

type TriageState string

const (
	TriageStateWaiting  TriageState = "waiting"
	TriageStateAccepted TriageState = "accepted"
	TriageStateDeclined TriageState = "declined"
	TriageStateMerged   TriageState = "merged"
)

type TriageDeclineReason string

const (
	TriageDeclineNotReproducible   TriageDeclineReason = "not_reproducible"
	TriageDeclineWorkingAsIntended TriageDeclineReason = "working_as_intended"
	TriageDeclineOutOfScope        TriageDeclineReason = "out_of_scope"
	TriageDeclineNoResponse        TriageDeclineReason = "no_response"
)

const TriageDeclineNoteMaxLen = 2000

func TriageDeclineReasons() []TriageDeclineReason {
	return []TriageDeclineReason{
		TriageDeclineNotReproducible,
		TriageDeclineWorkingAsIntended,
		TriageDeclineOutOfScope,
		TriageDeclineNoResponse,
	}
}

func (r TriageDeclineReason) Valid() bool {
	return slices.Contains(TriageDeclineReasons(), r)
}

func TriageStates() []TriageState {
	return []TriageState{
		TriageStateWaiting,
		TriageStateAccepted,
		TriageStateDeclined,
		TriageStateMerged,
	}
}

func (t TriageState) Valid() bool {
	return slices.Contains(TriageStates(), t)
}

func (t TriageState) Waiting() bool {
	return t == TriageStateWaiting
}

func (t TriageState) Terminal() bool {
	return t.Valid() && t != TriageStateWaiting
}

type TriageSettings struct {
	TeamID            uuid.UUID
	WorkspaceID       uuid.UUID
	RouteAgents       bool
	RouteIntegrations bool
	RouteNonMembers   bool
}

func (s TriageSettings) Routes(source ActorKind, onTeam bool) bool {
	switch source {
	case ActorKindAgent:
		return s.RouteAgents
	case ActorKindToken:
		return s.RouteIntegrations
	default:
		return s.RouteNonMembers && !onTeam
	}
}

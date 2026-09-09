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

type TriageSource string

const (
	TriageSourceUser  TriageSource = "user"
	TriageSourceToken TriageSource = "token"
	TriageSourceAgent TriageSource = "agent"
	TriageSourceEmail TriageSource = "email"
)

func TriageSources() []TriageSource {
	return []TriageSource{TriageSourceUser, TriageSourceToken, TriageSourceAgent, TriageSourceEmail}
}

func (s TriageSource) Valid() bool {
	return slices.Contains(TriageSources(), s)
}

func TriageSourceOf(kind ActorKind) TriageSource {
	switch kind {
	case ActorKindToken:
		return TriageSourceToken
	case ActorKindAgent:
		return TriageSourceAgent
	default:
		return TriageSourceUser
	}
}

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

func (s TriageSettings) Routes(source TriageSource, onTeam bool) bool {
	switch source {
	case TriageSourceAgent:
		return s.RouteAgents
	case TriageSourceToken:
		return s.RouteIntegrations
	case TriageSourceEmail:
		return true
	default:
		return s.RouteNonMembers && !onTeam
	}
}

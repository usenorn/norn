package entity

import (
	"errors"
	"slices"

	"github.com/google/uuid"
)

var ErrIssueDecisionForbidden = errors.New("only the assignee or a workspace admin can decide this")

type DecisionChannel string

const (
	DecisionChannelNorn     DecisionChannel = "norn"
	DecisionChannelTelegram DecisionChannel = "telegram"
)

func DecisionChannels() []DecisionChannel {
	return []DecisionChannel{DecisionChannelNorn, DecisionChannelTelegram}
}

func (c DecisionChannel) Valid() bool {
	return slices.Contains(DecisionChannels(), c)
}

type DecisionAuthority struct {
	AssigneeAccountID  uuid.UUID
	AssigneeKind       AccountKind
	DelegatorAccountID uuid.UUID
}

func (a DecisionAuthority) Maker() uuid.UUID {
	if a.AssigneeAccountID != uuid.Nil && a.AssigneeKind == AccountKindPerson {
		return a.AssigneeAccountID
	}

	return a.DelegatorAccountID
}

func (a DecisionAuthority) Permits(decision Decision) bool {
	if decision.Actor.AgentID != nil {
		return false
	}

	if decision.Role == MembershipRoleAdmin {
		return true
	}

	maker := a.Maker()

	return maker != uuid.Nil && decision.Actor.AccountID == maker
}

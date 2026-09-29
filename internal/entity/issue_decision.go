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
	AssigneeName       string
	DelegatorAccountID uuid.UUID
	DelegatorName      string
}

func (a DecisionAuthority) assigneeDecides() bool {
	return a.AssigneeAccountID != uuid.Nil && a.AssigneeKind == AccountKindPerson
}

func (a DecisionAuthority) Maker() uuid.UUID {
	if a.assigneeDecides() {
		return a.AssigneeAccountID
	}

	return a.DelegatorAccountID
}

func (a DecisionAuthority) MakerName() string {
	if a.assigneeDecides() {
		return a.AssigneeName
	}

	return a.DelegatorName
}

func (a DecisionAuthority) Permits(decision Decision) bool {
	if decision.Actor.AgentID != nil || decision.Role == MembershipRoleViewer {
		return false
	}

	if decision.Role == MembershipRoleAdmin {
		return true
	}

	maker := a.Maker()

	return maker != uuid.Nil && decision.Actor.AccountID == maker
}

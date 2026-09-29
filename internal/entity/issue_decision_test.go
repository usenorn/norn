package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestTheAssigneeDecidesUnlessTheyAreAnAgent(t *testing.T) {
	assignee := uuid.New()
	delegator := uuid.New()

	cases := []struct {
		name      string
		authority entity.DecisionAuthority
		want      uuid.UUID
	}{
		{
			name:      "a person assignee",
			authority: entity.DecisionAuthority{AssigneeAccountID: assignee, AssigneeKind: entity.AccountKindPerson, DelegatorAccountID: delegator},
			want:      assignee,
		},
		{
			name:      "an agent assignee falls back to the delegator",
			authority: entity.DecisionAuthority{AssigneeAccountID: assignee, AssigneeKind: entity.AccountKindAgent, DelegatorAccountID: delegator},
			want:      delegator,
		},
		{
			name:      "an unassigned issue falls back to the delegator",
			authority: entity.DecisionAuthority{DelegatorAccountID: delegator},
			want:      delegator,
		},
		{
			name:      "an agent assignee with nobody delegating leaves only admins",
			authority: entity.DecisionAuthority{AssigneeAccountID: assignee, AssigneeKind: entity.AccountKindAgent},
			want:      uuid.Nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.authority.Maker(); got != tc.want {
				t.Fatalf("maker is %s, want %s", got, tc.want)
			}
		})
	}
}

func TestOnlyTheDecisionMakerOrAnAdminMayDecide(t *testing.T) {
	assignee := uuid.New()
	agent := uuid.New()
	authority := entity.DecisionAuthority{AssigneeAccountID: assignee, AssigneeKind: entity.AccountKindPerson}

	cases := []struct {
		name     string
		decision entity.Decision
		want     bool
	}{
		{
			name:     "the assignee in a session",
			decision: entity.Decision{Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: assignee}, Role: entity.MembershipRoleMember},
			want:     true,
		},
		{
			name:     "the assignee through a linked telegram account",
			decision: entity.Decision{Actor: entity.Actor{Kind: entity.ActorKindToken, AccountID: assignee}, Role: entity.MembershipRoleMember},
			want:     true,
		},
		{
			name:     "a workspace admin",
			decision: entity.Decision{Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, Role: entity.MembershipRoleAdmin},
			want:     true,
		},
		{
			name:     "another member who manages the issue",
			decision: entity.Decision{Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, Role: entity.MembershipRoleMember},
			want:     false,
		},
		{
			name:     "an agent, even an admin one",
			decision: entity.Decision{Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New(), AgentID: &agent}, Role: entity.MembershipRoleAdmin},
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := authority.Permits(tc.decision); got != tc.want {
				t.Fatalf("permits is %t, want %t", got, tc.want)
			}
		})
	}
}

func TestNobodyButAnAdminDecidesWhenThereIsNoDecisionMaker(t *testing.T) {
	var authority entity.DecisionAuthority

	member := entity.Decision{Actor: entity.Actor{Kind: entity.ActorKindUser}, Role: entity.MembershipRoleMember}
	if authority.Permits(member) {
		t.Fatal("an actor without an account matched an absent decision maker")
	}
}

func TestOnlyTheChannelsTheSchemaAllowsAreValid(t *testing.T) {
	for _, channel := range entity.DecisionChannels() {
		if !channel.Valid() {
			t.Errorf("%q is offered as a channel but does not pass its own check", channel)
		}
	}

	if entity.DecisionChannel("email").Valid() {
		t.Error("a channel the table's check constraint would refuse passed validation")
	}
}

package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnAgentIsBoundedByItsOwnerRatherThanItself(t *testing.T) {
	agentAccount, owner := uuid.New(), uuid.New()

	actor := entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      agentAccount,
		OwnerAccountID: owner,
	}

	if actor.Authority() != owner {
		t.Fatal(
			"an agent's permissions were resolved against its own account. Every bound the " +
				"requirements ask for depends on this reading the owner instead.",
		)
	}

	if actor.AccountID != agentAccount {
		t.Error("resolving authority against the owner must not change who the actor is")
	}
}

func TestEverybodyElseIsTheirOwnAuthority(t *testing.T) {
	for name, actor := range map[string]entity.Actor{
		"a person":  {Kind: entity.ActorKindUser, AccountID: uuid.New()},
		"a token":   {Kind: entity.ActorKindToken, AccountID: uuid.New()},
		"anonymous": {},
	} {
		if actor.Authority() != actor.AccountID {
			t.Errorf("%s was given an authority other than itself", name)
		}
	}
}

func TestAnAgentActionIsAttributedToTheAgentNotItsOwner(t *testing.T) {
	agentAccount, owner := uuid.New(), uuid.New()

	decision := entity.Decision{Actor: entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      agentAccount,
		OwnerAccountID: owner,
	}}

	attribution := decision.ActivityActor()

	if attribution.AccountID != agentAccount {
		t.Fatal(
			"an agent's action was recorded against its owner. Making agents legible is the " +
				"point of the feature; a record naming the person hides which one acted.",
		)
	}

	if attribution.Kind != entity.ActorKindAgent {
		t.Errorf("attribution kind = %q, want agent", attribution.Kind)
	}
}

func TestAnAgentWithoutAnExplicitAllowanceIsStillBounded(t *testing.T) {
	if (entity.Agent{}).Allowance() != entity.AgentActionsPerWindow {
		t.Fatal("an agent registered without a limit was left unbounded")
	}

	zero := 0
	if (entity.Agent{ActionLimit: &zero}).Allowance() != entity.AgentActionsPerWindow {
		t.Error("a zero limit must mean the default, never no limit at all")
	}

	ten := 10
	if got := (entity.Agent{ActionLimit: &ten}).Allowance(); got != ten {
		t.Errorf("allowance = %d, want the configured 10", got)
	}
}

func TestOnlyTheActionsATeamNamedAreHeld(t *testing.T) {
	settings := entity.AgentSettings{HoldStateChanges: true}

	if !settings.Holds(entity.AgentActionStateChange) {
		t.Error("a held action was let through")
	}

	for _, action := range []entity.AgentAction{
		entity.AgentActionComment, entity.AgentActionIssueEdit,
	} {
		if settings.Holds(action) {
			t.Errorf("%q was held although the team only asked to hold state changes", action)
		}
	}
}

func TestATeamThatHasNeverConfiguredAgentsHoldsNothing(t *testing.T) {
	settings := entity.AgentSettings{}

	for _, action := range entity.AgentActions() {
		if settings.Holds(action) {
			t.Fatalf(
				"%q was held by a team with no agent settings. Absence must mean nothing is "+
					"held, or adding the feature would silently block every agent everywhere.",
				action,
			)
		}
	}
}

func TestAProposalIsDecidedOnceAndNeverReopened(t *testing.T) {
	for _, target := range []entity.AgentProposalStatus{
		entity.AgentProposalApplied, entity.AgentProposalRejected, entity.AgentProposalFailed,
	} {
		if !entity.AgentProposalPending.CanTransitionTo(target) {
			t.Errorf("a pending proposal could not move to %q", target)
		}

		if target.CanTransitionTo(entity.AgentProposalPending) {
			t.Errorf("%q was allowed back to pending, so a decision could be undone", target)
		}

		if target.CanTransitionTo(entity.AgentProposalApplied) {
			t.Errorf("%q could still be applied, so one approval could act twice", target)
		}
	}
}

func TestARateLimitedAgentIsToldRatherThanConcealed(t *testing.T) {
	if !entity.DenyReasonAgentRateLimited.Disclosed() {
		t.Fatal(
			"an agent refused for acting too fast was not told why. It cannot back off from a " +
				"reason it never receives.",
		)
	}

	denied := entity.AccessDeniedError{Reason: entity.DenyReasonAgentRateLimited}

	if !errorsIs(denied, entity.ErrAgentRateLimited) {
		t.Error("a rate-limited denial did not surface as ErrAgentRateLimited")
	}
}

func errorsIs(err, target error) bool {
	type unwrapper interface{ Unwrap() error }

	if err == target {
		return true
	}

	if wrapped, ok := err.(unwrapper); ok {
		return wrapped.Unwrap() == target
	}

	return false
}

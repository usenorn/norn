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
	settings := entity.AgentSettings{HoldStateChanges: entity.AgentHoldAlways}

	if settings.Holds(entity.AgentActionStateChange) != entity.AgentHoldAlways {
		t.Error("a held action was let through")
	}

	for _, action := range []entity.AgentAction{
		entity.AgentActionComment, entity.AgentActionIssueEdit,
	} {
		if settings.Holds(action) != entity.AgentHoldNever {
			t.Errorf("%q was held although the team only asked to hold state changes", action)
		}
	}
}

func TestAWriteIsHeldByTheStrongestPolicyAnyFieldItChangesAsksFor(t *testing.T) {
	both := []entity.AgentAction{entity.AgentActionStateChange, entity.AgentActionIssueEdit}

	holdsState := entity.AgentSettings{
		HoldStateChanges: entity.AgentHoldAlways,
		HoldIssueEdits:   entity.AgentHoldNever,
	}

	if action, hold := holdsState.Strongest(both); hold != entity.AgentHoldAlways ||
		action != entity.AgentActionStateChange {
		t.Fatalf(
			"a write that moves state and renames was classified as %q/%q; renaming in the same "+
				"call must not carry a close past a hold",
			action, hold,
		)
	}

	holdsEdits := entity.AgentSettings{
		HoldStateChanges: entity.AgentHoldNever,
		HoldIssueEdits:   entity.AgentHoldAlways,
	}

	if _, hold := holdsEdits.Strongest(both); hold != entity.AgentHoldAlways {
		t.Fatal("a team that holds edits let an edit through because it also moved state")
	}
}

func TestHoldPoliciesAreOrderedFromWeakestToStrongest(t *testing.T) {
	if !entity.AgentHoldAlways.Stronger(entity.AgentHoldNever) {
		t.Fatal("hold policies are not ordered, so the strongest one cannot be chosen")
	}
}

func TestATeamThatHasNeverConfiguredAgentsReadsBackAsHoldingNothing(t *testing.T) {
	settings := entity.AgentSettings{}.Normalised()

	for _, hold := range []entity.AgentHold{
		settings.HoldComments, settings.HoldStateChanges, settings.HoldIssueEdits,
	} {
		if hold != entity.AgentHoldNever {
			t.Fatalf(
				"an unconfigured setting reads back as %q. It has to be a real value, or the "+
					"screen showing it sends that empty value straight back and is refused.",
				hold,
			)
		}
	}
}

func TestATeamThatHasNeverConfiguredAgentsHoldsNothingItCanChoose(t *testing.T) {
	settings := entity.AgentSettings{}

	for _, action := range []entity.AgentAction{
		entity.AgentActionComment, entity.AgentActionStateChange, entity.AgentActionIssueEdit,
	} {
		if settings.Holds(action) != entity.AgentHoldNever {
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

func TestAnAgentIsManagedByWhoeverItActsForOrAnAdministrator(t *testing.T) {
	owner, stranger := uuid.New(), uuid.New()
	agent := entity.Agent{OwnerAccountID: owner}

	for name, probe := range map[string]struct {
		accountID uuid.UUID
		role      entity.MembershipRole
		allowed   bool
	}{
		"the person it acts for": {owner, entity.MembershipRoleMember, true},
		"another member":         {stranger, entity.MembershipRoleMember, false},
		"an administrator":       {stranger, entity.MembershipRoleAdmin, true},
		"a viewer who owns it":   {owner, entity.MembershipRoleViewer, true},
		"a viewer who does not":  {stranger, entity.MembershipRoleViewer, false},
	} {
		if got := agent.ManageableBy(probe.accountID, probe.role); got != probe.allowed {
			t.Errorf("%s managing the agent: allowed = %v, want %v", name, got, probe.allowed)
		}
	}
}

func TestAnUnownedAgentBelongsToNobody(t *testing.T) {
	agent := entity.Agent{}

	if agent.OwnedBy(uuid.Nil) {
		t.Error(
			"an agent with no owner recorded was claimed by an account with no id. Every " +
				"caller without an account would inherit it.",
		)
	}
}

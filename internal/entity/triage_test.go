package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestWhatEntersTriageIsDecidedByWhoFiledItAndNothingElse(t *testing.T) {
	all := entity.TriageSettings{RouteAgents: true, RouteIntegrations: true, RouteNonMembers: true}
	none := entity.TriageSettings{}

	for name, expected := range map[string]struct {
		settings entity.TriageSettings
		source   entity.ActorKind
		onTeam   bool
		routed   bool
	}{
		"an agent, when agents are routed": {all, entity.ActorKindAgent, false, true},
		"an agent on the team, when agents are routed": {
			all, entity.ActorKindAgent, true, true,
		},
		"an agent, when agents are not routed": {
			entity.TriageSettings{RouteIntegrations: true, RouteNonMembers: true},
			entity.ActorKindAgent, false, false,
		},
		"an integration, when integrations are routed": {all, entity.ActorKindToken, false, true},
		"an integration, when integrations are not routed": {
			entity.TriageSettings{RouteAgents: true, RouteNonMembers: true},
			entity.ActorKindToken, false, false,
		},
		"someone off the team, when outsiders are routed": {all, entity.ActorKindUser, false, true},
		"someone on the team, when outsiders are routed":  {all, entity.ActorKindUser, true, false},
		"someone off the team, when outsiders are trusted": {
			entity.TriageSettings{RouteAgents: true, RouteIntegrations: true},
			entity.ActorKindUser, false, false,
		},
		"anyone at all, when nothing is routed":  {none, entity.ActorKindUser, false, false},
		"an agent, when nothing is routed":       {none, entity.ActorKindAgent, false, false},
		"an integration, when nothing is routed": {none, entity.ActorKindToken, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			if routed := expected.settings.Routes(expected.source, expected.onTeam); routed != expected.routed {
				t.Fatalf(
					"routed=%v, want %v. Whether work waits for a decision is the team's call about "+
						"who filed it; getting it wrong either floods the backlog or buries work "+
						"nobody meant to hold up.",
					routed, expected.routed,
				)
			}
		})
	}
}

func TestAnAgentOnTheTeamIsStillAnAgent(t *testing.T) {
	settings := entity.TriageSettings{RouteAgents: true}

	if !settings.Routes(entity.ActorKindAgent, true) {
		t.Fatal(
			"an agent that happens to be on the team skipped triage. Membership is what makes a " +
				"person trusted; an agent is held for review because of what it is, and adding it " +
				"to the team must not be a way around that.",
		)
	}
}

func TestOnlyWaitingIsNotADecision(t *testing.T) {
	if entity.TriageStateWaiting.Terminal() {
		t.Error("waiting reads as a decision, so an undecided issue would look settled")
	}

	for _, state := range []entity.TriageState{
		entity.TriageStateAccepted, entity.TriageStateDeclined, entity.TriageStateMerged,
	} {
		if !state.Terminal() {
			t.Errorf("%q does not read as a decision, so it would stay in the queue after being decided", state)
		}
	}

	if entity.TriageState("").Terminal() || entity.TriageState("").Waiting() {
		t.Error(
			"an issue that never entered triage reads as waiting or decided. It did neither, and " +
				"the queue must not pick it up.",
		)
	}
}

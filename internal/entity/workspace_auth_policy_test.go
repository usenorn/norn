package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestSSOIsEnforcedEverywhereOnlyWhenEveryWorkspaceSaysSo(t *testing.T) {
	cases := []struct {
		name         string
		enforcements []entity.AuthEnforcement
		enforced     bool
	}{
		{"no workspaces", nil, false},
		{"every workspace enforces sso", []entity.AuthEnforcement{entity.AuthEnforcementSSO, entity.AuthEnforcementSSO}, true},
		{"one workspace still accepts passwords", []entity.AuthEnforcement{entity.AuthEnforcementSSO, entity.AuthEnforcementAny}, false},
		{"no workspace enforces sso", []entity.AuthEnforcement{entity.AuthEnforcementAny}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.SSOEnforcedEverywhere(c.enforcements); got != c.enforced {
				t.Fatalf("SSOEnforcedEverywhere(%v) = %v, want %v", c.enforcements, got, c.enforced)
			}
		})
	}
}

func TestOnlyMachineActorsSkipTheEnforcementCheck(t *testing.T) {
	workspaceID := uuid.New()

	for name, tc := range map[string]struct {
		actor   entity.Actor
		permits bool
	}{
		"an API token has no method to narrow": {
			entity.Actor{Kind: entity.ActorKindToken, AccountID: uuid.New()}, true,
		},
		"an agent has no method to narrow": {
			entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}, true,
		},
		"a password session is refused": {
			entity.Actor{
				Kind:       entity.ActorKindUser,
				AccountID:  uuid.New(),
				AuthMethod: entity.SessionAuthMethodPassword,
			}, false,
		},
		"an SSO session from this workspace is admitted": {
			entity.Actor{
				Kind:           entity.ActorKindUser,
				AccountID:      uuid.New(),
				AuthMethod:     entity.SessionAuthMethodSSO,
				SSOWorkspaceID: workspaceID,
			}, true,
		},
		"an SSO session from another workspace is refused": {
			entity.Actor{
				Kind:           entity.ActorKindUser,
				AccountID:      uuid.New(),
				AuthMethod:     entity.SessionAuthMethodSSO,
				SSOWorkspaceID: uuid.New(),
			}, false,
		},
		"somebody joining anonymously is refused": {
			entity.Actor{}, false,
		},
	} {
		if got := entity.AuthEnforcementSSO.PermitsActor(tc.actor, workspaceID); got != tc.permits {
			t.Errorf("%s: PermitsActor = %v, want %v", name, got, tc.permits)
		}
	}
}

func TestWorkRequestedEarlierStillRunsWhenTheWorkspaceRequiresSSO(t *testing.T) {
	workspaceID := uuid.New()
	authority := entity.AuthorityOf(entity.Actor{}, workspaceID)

	replayed := authority.Replay(
		entity.ActorKindUser,
		uuid.New(),
		entity.SessionAuthMethodSSO,
		workspaceID,
	)

	if !entity.AuthEnforcementSSO.PermitsActor(replayed, workspaceID) {
		t.Fatal(
			"an import or bulk action replaying the actor that asked for it was refused by the " +
				"workspace it belongs to. Replay reconstructs the workspace the request was made " +
				"against, so it has to satisfy the same check a live session would.",
		)
	}

	if entity.AuthEnforcementSSO.PermitsActor(replayed, uuid.New()) {
		t.Fatal(
			"a replayed actor reached a workspace its request was never made against. Replay must " +
				"not be a way around the cross-workspace check a live session is held to.",
		)
	}
}

func TestReplayCarriesOnlyTheTeamsTheRequestWasAuthorisedFor(t *testing.T) {
	workspaceID, teamID := uuid.New(), uuid.New()

	confined := entity.Actor{
		Kind:      entity.ActorKindToken,
		AccountID: uuid.New(),
		Grants: entity.APITokenGrants{{
			WorkspaceID: workspaceID,
			TeamIDs:     []uuid.UUID{teamID},
		}},
		Scopes: entity.APIScopeSet{entity.NewAPIScope(entity.ResourceIssue, entity.ActionUpdate)},
	}

	replayed := entity.AuthorityOf(confined, workspaceID).
		Replay(entity.ActorKindToken, confined.AccountID, "", workspaceID)

	if replayed.ConfinedTo(uuid.New()) {
		t.Fatal(
			"work replayed from a stored request reached a workspace the token was never granted. " +
				"A token confined to one workspace could start an import and have it touch every " +
				"other one the account belongs to.",
		)
	}

	scope := replayed.NarrowScope(entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	if scope.AllTeams || len(scope.TeamIDs) != 1 || scope.TeamIDs[0] != teamID {
		t.Fatalf(
			"replayed scope is %+v. A token confined to one team must not widen to every team in "+
				"the workspace once its work runs asynchronously.",
			scope,
		)
	}

	if replayed.Holds(entity.NewPermission(entity.ResourceLabel, entity.ActionManage)) {
		t.Fatal(
			"the replayed actor held a permission the token never had, so an import could write " +
				"resources the caller was not scoped for.",
		)
	}
}

func TestSomebodyJoiningAnonymouslyIsStillAdmittedWhereAnyMethodIsAccepted(t *testing.T) {
	if !entity.AuthEnforcementAny.PermitsActor(entity.Actor{}, uuid.New()) {
		t.Fatal(
			"an anonymous invitee was refused by a workspace that accepts any method. Accepting " +
				"an invitation is the one flow that legitimately has no actor yet.",
		)
	}
}

func TestAnApprovedAgentActionCarriesOnlyWhatThatActionNeeds(t *testing.T) {
	for name, tc := range map[string]struct {
		action  entity.AgentAction
		granted entity.Permission
		denied  entity.Permission
	}{
		"a comment": {
			action:  entity.AgentActionComment,
			granted: entity.NewPermission(entity.ResourceComment, entity.ActionManage),
			denied:  entity.NewPermission(entity.ResourceIssue, entity.ActionUpdate),
		},
		"a state change": {
			action:  entity.AgentActionStateChange,
			granted: entity.NewPermission(entity.ResourceIssue, entity.ActionUpdate),
			denied:  entity.NewPermission(entity.ResourceIssue, entity.ActionDelete),
		},
		"an issue edit": {
			action:  entity.AgentActionIssueEdit,
			granted: entity.NewPermission(entity.ResourceIssue, entity.ActionUpdate),
			denied:  entity.NewPermission(entity.ResourceAPIToken, entity.ActionManage),
		},
	} {
		acting := entity.Actor{Kind: entity.ActorKindAgent, Scopes: tc.action.Scopes()}

		if !acting.Holds(tc.granted) {
			t.Errorf("%s: approving it did not carry the permission the action needs", name)
		}

		if acting.Holds(tc.denied) {
			t.Errorf(
				"%s: approving one action carried a permission it never needed. A person approves "+
					"one described change, not everything the agent could otherwise do.",
				name,
			)
		}
	}
}

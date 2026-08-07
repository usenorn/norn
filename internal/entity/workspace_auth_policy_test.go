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
	replayed := entity.Actor{
		Kind:       entity.ActorKindUser,
		AccountID:  uuid.New(),
		AuthMethod: entity.SessionAuthMethodSSO,
	}

	if !entity.AuthEnforcementSSO.PermitsActor(replayed, uuid.New()) {
		t.Fatal(
			"an import, bulk action or approval replaying the actor that asked for it was refused. " +
				"Those are rebuilt from what was stored about the request, so they carry no session " +
				"and cannot name the workspace its provider signed into.",
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

package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestATokenReachesOnlyTheWorkspacesItWasGranted(t *testing.T) {
	granted, other := uuid.New(), uuid.New()

	actor := entity.Actor{
		Kind:      entity.ActorKindToken,
		AccountID: uuid.New(),
		Grants:    entity.APITokenGrants{{WorkspaceID: granted, AllTeams: true}},
	}

	if !actor.ConfinedTo(granted) {
		t.Error("a token was refused the workspace it was granted")
	}

	if actor.ConfinedTo(other) {
		t.Error(
			"a token reached a workspace it was never granted. The owner belonging to a " +
				"workspace is not the same as the token being allowed to act in it.",
		)
	}
}

func TestASessionActorIsConfinedByMembershipRatherThanByGrants(t *testing.T) {
	actor := entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}

	if !actor.ConfinedTo(uuid.New()) {
		t.Fatal(
			"a session actor was treated as confined. Only tokens carry grants; a person is " +
				"confined by the membership lookup that follows.",
		)
	}
}

func TestAGrantNarrowsTheOwnersTeamsAndNeverWidensThem(t *testing.T) {
	workspace := uuid.New()
	onTeam, alsoOnTeam, notOnTeam := uuid.New(), uuid.New(), uuid.New()

	resolved := entity.TeamScope{
		WorkspaceID: workspace,
		TeamIDs:     []uuid.UUID{onTeam, alsoOnTeam},
	}

	actor := entity.Actor{
		Grants: entity.APITokenGrants{{
			WorkspaceID: workspace,
			TeamIDs:     []uuid.UUID{onTeam, notOnTeam},
		}},
	}

	narrowed := actor.NarrowScope(resolved)

	if !narrowed.Covers(onTeam) {
		t.Error("a team both the owner and the grant name was dropped")
	}

	if narrowed.Covers(alsoOnTeam) {
		t.Error("a team the grant did not name was still reachable")
	}

	if narrowed.Covers(notOnTeam) {
		t.Fatal(
			"a grant naming a team its owner cannot see made that team reachable. A grant may " +
				"only ever subtract from what the owner may do.",
		)
	}
}

func TestAnAllTeamsGrantLeavesTheOwnersScopeExactlyAsResolved(t *testing.T) {
	workspace, team := uuid.New(), uuid.New()

	resolved := entity.TeamScope{WorkspaceID: workspace, AllTeams: true, IncludePrivate: true}

	actor := entity.Actor{
		Grants: entity.APITokenGrants{{WorkspaceID: workspace, AllTeams: true}},
	}

	narrowed := actor.NarrowScope(resolved)

	if !narrowed.AllTeams || !narrowed.IncludePrivate || !narrowed.Covers(team) {
		t.Fatal("an all-teams grant narrowed a scope it should have passed through untouched")
	}
}

func TestPinningATokenToTeamsKeepsItsAbilityToSeePrivateOnes(t *testing.T) {
	workspace, private := uuid.New(), uuid.New()

	resolved := entity.TeamScope{WorkspaceID: workspace, AllTeams: true, IncludePrivate: true}

	actor := entity.Actor{
		Grants: entity.APITokenGrants{{WorkspaceID: workspace, TeamIDs: []uuid.UUID{private}}},
	}

	narrowed := actor.NarrowScope(resolved)

	if !narrowed.IncludePrivate {
		t.Fatal(
			"pinning an admin's token to named teams also took away its ability to see private " +
				"ones. Reaching every team and being allowed to see private teams are separate " +
				"questions, and narrowing the first must not answer the second.",
		)
	}

	if !narrowed.Covers(private) {
		t.Error("the pinned team was not reachable")
	}
}

func TestTheMintCeilingMatchesWhatTheRoleMayActuallyDo(t *testing.T) {
	for _, probe := range []struct {
		role     entity.MembershipRole
		scope    entity.APIScope
		mintable bool
	}{
		{entity.MembershipRoleAdmin, entity.NewAPIScope(entity.ResourceIssue, entity.ActionManage), true},
		{entity.MembershipRoleMember, entity.NewAPIScope(entity.ResourceIssue, entity.ActionRead), true},
		{entity.MembershipRoleViewer, entity.NewAPIScope(entity.ResourceIssue, entity.ActionRead), true},
		{entity.MembershipRoleMember, entity.NewAPIScope(entity.ResourceIssue, entity.ActionManage), false},
		{entity.MembershipRoleMember, entity.NewAPIScope(entity.ResourceProject, entity.ActionManage), true},
		{entity.MembershipRoleViewer, entity.NewAPIScope(entity.ResourceProject, entity.ActionManage), false},
	} {
		allowed := entity.AllowedAPIScopesFor(probe.role)

		if got := allowed.Permits(probe.scope.Resource(), probe.scope.Action()); got != probe.mintable {
			t.Errorf(
				"a %s minting %q: allowed = %v, want %v (runtime policy says %v)",
				probe.role, probe.scope, got, probe.mintable,
				entity.RoleGrants(probe.role, probe.scope.Resource(), probe.scope.Action()),
			)
		}
	}
}

func TestEveryMintableScopeIsSomethingTheRoleMayActuallyDo(t *testing.T) {
	for _, role := range []entity.MembershipRole{
		entity.MembershipRoleAdmin, entity.MembershipRoleMember, entity.MembershipRoleViewer,
	} {
		for _, scope := range entity.AllowedAPIScopesFor(role) {
			if !entity.RoleGrants(role, scope.Resource(), scope.Action()) {
				t.Errorf(
					"a %s may mint %q but the policy denies it. A token would carry a scope that "+
						"can never be exercised.",
					role, scope,
				)
			}
		}
	}
}

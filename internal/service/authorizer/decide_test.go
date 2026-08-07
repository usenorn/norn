package authorizer

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
)

type stubEnforcer struct {
	allow bool
	calls int
}

func (s *stubEnforcer) Enforce(...any) (bool, error) {
	s.calls++

	return s.allow, nil
}

func (s *stubEnforcer) GetPolicy() ([][]string, error) { return nil, nil }

func (s *stubEnforcer) AddPolicies([][]string) (bool, error) { return true, nil }

func (s *stubEnforcer) RemovePolicies([][]string) (bool, error) { return true, nil }

func actingAs(accountID uuid.UUID, method entity.SessionAuthMethod) context.Context {
	return identity.WithActor(context.Background(), entity.Actor{
		Kind:       entity.ActorKindUser,
		AccountID:  accountID,
		AuthMethod: method,
	})
}

func actingAsSSOFrom(accountID, workspaceID uuid.UUID) context.Context {
	return identity.WithActor(context.Background(), entity.Actor{
		Kind:           entity.ActorKindUser,
		AccountID:      accountID,
		AuthMethod:     entity.SessionAuthMethodSSO,
		SSOWorkspaceID: workspaceID,
	})
}

func TestAnActionWithNoExplicitRuleIsDenied(t *testing.T) {
	enforcer := &stubEnforcer{allow: false}
	harness := newDecider(t, enforcer, entity.MembershipRoleAdmin, entity.AuthEnforcementAny)

	_, err := harness.Decide(actingAs(uuid.New(), entity.SessionAuthMethodPassword), entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionManage,
		WorkspaceID: uuid.New(),
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("Decide error = %v, want an AccessDeniedError", err)
	}

	if denied.Reason != entity.DenyReasonRoleLacksAction {
		t.Fatalf("reason = %q, want role_lacks_action", denied.Reason)
	}
}

func TestARequestWithoutAnActorIsDenied(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementAny)

	_, err := harness.Decide(context.Background(), entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionRead,
		WorkspaceID: uuid.New(),
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonNoActor {
		t.Fatalf("Decide error = %v, want a no_actor denial", err)
	}
}

func TestANonMemberIsDeniedWithoutConsultingThePolicy(t *testing.T) {
	enforcer := &stubEnforcer{allow: true}
	harness := newDeciderWithMembershipError(t, enforcer, entity.ErrMembershipNotFound)

	_, err := harness.Decide(actingAs(uuid.New(), entity.SessionAuthMethodPassword), entity.AccessRequest{
		Resource:    entity.ResourceMembership,
		Action:      entity.ActionRead,
		WorkspaceID: uuid.New(),
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonNotAMember {
		t.Fatalf("Decide error = %v, want a not_a_member denial", err)
	}

	if enforcer.calls != 0 {
		t.Fatal("a non-member must be refused before the role policy is consulted")
	}
}

func TestAConcealingResourceReportsNotFoundRatherThanForbidden(t *testing.T) {
	cases := map[entity.Resource]error{
		entity.ResourceTeam:       entity.ErrTeamNotFound,
		entity.ResourceIssue:      entity.ErrIssueNotFound,
		entity.ResourceMembership: entity.ErrAccountForbidden,
	}

	for resource, want := range cases {
		t.Run(string(resource), func(t *testing.T) {
			denied := entity.AccessDeniedError{Reason: entity.DenyReasonNotAMember, Resource: resource}

			if !errors.Is(denied, want) {
				t.Fatalf("%s surfaced as %v, want %v", resource, error(denied), want)
			}

			if resource.Conceals() && errors.Is(denied, entity.ErrAccountForbidden) {
				t.Fatal("a concealed resource must not answer forbidden, which would confirm it exists")
			}
		})
	}
}

func TestOnlyActionableReasonsAreDisclosed(t *testing.T) {
	disclosed := map[entity.DenyReason]bool{
		entity.DenyReasonRoleLacksAction:        true,
		entity.DenyReasonAuthMethodNotPermitted: true,
		entity.DenyReasonTokenPermissionMissing: true,
		entity.DenyReasonTokenWorkspaceMismatch: true,
		entity.DenyReasonInstanceAdminRequired:  true,
		entity.DenyReasonNotAMember:             false,
		entity.DenyReasonNoActor:                false,
		entity.DenyReasonNotSelf:                false,
		entity.DenyReasonUnknownRole:            false,
		entity.DenyReasonWorkspaceDeleted:       false,
	}

	for reason, want := range disclosed {
		if got := reason.Disclosed(); got != want {
			t.Errorf("%q disclosed = %t, want %t", reason, got, want)
		}
	}
}

func TestATokenIsConfinedToTheWorkspaceItWasMintedFor(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementAny)

	minted := uuid.New()
	other := uuid.New()
	tokenID := uuid.New()

	ctx := identity.WithActor(context.Background(), entity.Actor{
		Kind:      entity.ActorKindToken,
		AccountID: uuid.New(),
		TokenID:   &tokenID,
		Grants:    entity.APITokenGrants{{WorkspaceID: minted, AllTeams: true}},
		Scopes:    entity.APIScopeSet{entity.NewAPIScope(entity.ResourceTeam, entity.ActionRead)},
	})

	_, err := harness.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionRead,
		WorkspaceID: other,
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonTokenWorkspaceMismatch {
		t.Fatalf("Decide error = %v, want a token_workspace_mismatch denial", err)
	}
}

func TestATokenIsRefusedAnActionOutsideItsScopes(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementAny)

	workspaceID := uuid.New()
	tokenID := uuid.New()

	ctx := identity.WithActor(context.Background(), entity.Actor{
		Kind:      entity.ActorKindToken,
		AccountID: uuid.New(),
		TokenID:   &tokenID,
		Grants:    entity.APITokenGrants{{WorkspaceID: workspaceID, AllTeams: true}},
		Scopes:    entity.APIScopeSet{entity.NewAPIScope(entity.ResourceTeam, entity.ActionRead)},
	})

	_, err := harness.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonTokenPermissionMissing {
		t.Fatalf("Decide error = %v, want a token_permission_missing denial", err)
	}
}

func TestATokenNeverExceedsItsCreatingAccount(t *testing.T) {
	enforcer := &stubEnforcer{allow: false}
	harness := newDecider(t, enforcer, entity.MembershipRoleMember, entity.AuthEnforcementAny)

	workspaceID := uuid.New()
	tokenID := uuid.New()

	ctx := identity.WithActor(context.Background(), entity.Actor{
		Kind:      entity.ActorKindToken,
		AccountID: uuid.New(),
		TokenID:   &tokenID,
		Grants:    entity.APITokenGrants{{WorkspaceID: workspaceID, AllTeams: true}},
		Scopes:    entity.APIScopeSet{entity.NewAPIScope(entity.ResourceTeam, entity.ActionManage)},
	})

	_, err := harness.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonRoleLacksAction {
		t.Fatalf("Decide error = %v, want the role policy to refuse a token its account cannot use", err)
	}
}

func TestAPasswordActorIsRefusedByAnSSOEnforcingWorkspace(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementSSO)

	_, err := harness.Decide(actingAs(uuid.New(), entity.SessionAuthMethodPassword), entity.AccessRequest{
		Resource:    entity.ResourceMembership,
		Action:      entity.ActionRead,
		WorkspaceID: uuid.New(),
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonAuthMethodNotPermitted {
		t.Fatalf("Decide error = %v, want an auth_method_not_permitted denial", err)
	}
}

func TestAnSSOActorIsAdmittedByTheWorkspaceWhoseProviderSignedItIn(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementSSO)

	workspaceID := uuid.New()

	if _, err := harness.Decide(actingAsSSOFrom(uuid.New(), workspaceID), entity.AccessRequest{
		Resource:    entity.ResourceMembership,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		t.Fatalf("Decide: %v", err)
	}
}

func TestARequestNamingNeitherAWorkspaceNorASubjectIsRefused(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementAny)

	_, err := harness.Decide(actingAs(uuid.New(), entity.SessionAuthMethodPassword), entity.AccessRequest{
		Resource: entity.ResourceIssue,
		Action:   entity.ActionManage,
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf(
			"Decide allowed a request that named no workspace and no subject, giving %v. The self "+
				"check takes a caller-supplied uuid, so one zero value there would otherwise turn a "+
				"scoped check into an allow-all.",
			err,
		)
	}
}

func TestAnSSOSessionFromAnotherWorkspaceDoesNotSatisfyThisOne(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementSSO)

	_, err := harness.Decide(actingAsSSOFrom(uuid.New(), uuid.New()), entity.AccessRequest{
		Resource:    entity.ResourceMembership,
		Action:      entity.ActionRead,
		WorkspaceID: uuid.New(),
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonAuthMethodNotPermitted {
		t.Fatalf(
			"Decide error = %v, want an auth_method_not_permitted denial. Requiring the string "+
				"\"sso\" without asking whose provider issued it lets anyone who administers one "+
				"workspace satisfy another workspace's requirement.",
			err,
		)
	}
}

func TestTheAuthPolicyItselfIsNotNarrowedByTheAuthMethod(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementSSO)

	if _, err := harness.Decide(actingAs(uuid.New(), entity.SessionAuthMethodPassword), entity.AccessRequest{
		Resource:    entity.ResourceAuthPolicy,
		Action:      entity.ActionUpdate,
		WorkspaceID: uuid.New(),
	}); err != nil {
		t.Fatalf("an admin on a password session must still reach the policy screen, got %v", err)
	}
}

func TestATokenIsNotNarrowedByAnAuthMethodItCannotHave(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementSSO)

	workspaceID := uuid.New()
	tokenID := uuid.New()

	ctx := identity.WithActor(context.Background(), entity.Actor{
		Kind:      entity.ActorKindToken,
		AccountID: uuid.New(),
		TokenID:   &tokenID,
		Grants:    entity.APITokenGrants{{WorkspaceID: workspaceID, AllTeams: true}},
		Scopes:    entity.APIScopeSet{entity.NewAPIScope(entity.ResourceTeam, entity.ActionRead)},
	})

	if _, err := harness.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		t.Fatalf("a token carries no auth method to narrow, got %v", err)
	}
}

func TestAnAnonymousRequestIsDistinguishableFromARefusedOne(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementAny)

	_, err := harness.Decide(context.Background(), entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: uuid.New(),
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("Decide error = %v, want an AccessDeniedError", err)
	}

	if denied.Reason != entity.DenyReasonNoActor {
		t.Fatalf("reason = %q, want no_actor so the edge can answer 401 rather than hiding the resource", denied.Reason)
	}
}

func TestTheEnforcementDenialIsDistinguishableFromAnOrdinaryRefusal(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementSSO)

	_, err := harness.Decide(actingAs(uuid.New(), entity.SessionAuthMethodPassword), entity.AccessRequest{
		Resource:    entity.ResourceMembership,
		Action:      entity.ActionRead,
		WorkspaceID: uuid.New(),
	})

	if !errors.Is(err, entity.ErrWorkspaceAuthMethodNotPermitted) {
		t.Fatalf(
			"a refusal for the wrong authentication method came back as %v. A caller cannot tell "+
				"it apart from an ordinary permission refusal, so no screen can offer to sign the "+
				"person in through their provider.",
			err,
		)
	}

	if errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatal(
			"the enforcement denial still reads as a plain forbidden, so anything switching on " +
				"that sentinel would treat it as a role problem",
		)
	}
}

func TestTheSSOConnectionScreenIsNotNarrowedByTheAuthMethodEither(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementSSO)

	if _, err := harness.Decide(actingAs(uuid.New(), entity.SessionAuthMethodPassword), entity.AccessRequest{
		Resource:    entity.ResourceSSOConnection,
		Action:      entity.ActionUpdate,
		WorkspaceID: uuid.New(),
	}); err != nil {
		t.Fatalf(
			"an admin on a password session cannot reach the provider settings (%v). That is the "+
				"screen they need in order to repair a provider that has stopped working, so it "+
				"has to stay reachable exactly when enforcement is refusing everything else.",
			err,
		)
	}
}

func TestAnActorWithNoAuthMethodIsRefusedEvenWhereAnythingIsAccepted(t *testing.T) {
	harness := newDecider(t, &stubEnforcer{allow: true}, entity.MembershipRoleAdmin, entity.AuthEnforcementAny)

	_, err := harness.Decide(actingAs(uuid.New(), ""), entity.AccessRequest{
		Resource:    entity.ResourceMembership,
		Action:      entity.ActionRead,
		WorkspaceID: uuid.New(),
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonAuthMethodNotPermitted {
		t.Fatalf(
			"an actor claiming no authentication method at all was admitted to a workspace that "+
				"accepts any method (%v). Anything that builds an actor without setting the method "+
				"would silently gain access; refusing is the safe reading of an unknown method.",
			err,
		)
	}
}

package workspace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
)

func (h *harness) expectActorIsMember(workspaceID, actorID uuid.UUID) {
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, actorID).
		Return(entity.Membership{WorkspaceID: workspaceID, AccountID: actorID, Role: entity.MembershipRoleAdmin}, nil)
}

func TestAPasswordSessionIsRefusedByAnSSOEnforcingWorkspace(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorIsMember(workspaceID, actorID)
	h.expectAuthEnforcement(workspaceID, entity.AuthEnforcementSSO)

	_, err := h.service.ListMembers(actingAs(actorID), workspaceID)
	if !errors.Is(err, entity.ErrWorkspaceAuthMethodNotPermitted) {
		t.Fatalf("ListMembers error = %v, want ErrWorkspaceAuthMethodNotPermitted", err)
	}
}

func TestTheSameSessionStillReachesWorkspacesThatDoNotEnforceSSO(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	enforcing := uuid.New()
	permissive := uuid.New()

	session := passwordSession(actorID)
	ctx := identity.WithSession(context.Background(), session)

	h.expectActorIsMember(enforcing, actorID)
	h.expectAuthEnforcement(enforcing, entity.AuthEnforcementSSO)

	if _, err := h.service.ListMembers(ctx, enforcing); !errors.Is(err, entity.ErrWorkspaceAuthMethodNotPermitted) {
		t.Fatalf("the SSO-enforcing workspace should refuse this session, got %v", err)
	}

	h.expectActorIsMember(permissive, actorID)
	h.expectAuthEnforcement(permissive, entity.AuthEnforcementAny)
	h.authorizer.EXPECT().
		Authorize(gomock.Any(), entity.MembershipRoleAdmin, entity.ResourceMembership, entity.ActionRead).
		Return(nil)
	h.memberships.EXPECT().ListByWorkspaceID(gomock.Any(), permissive).Return(nil, nil)

	if _, err := h.service.ListMembers(ctx, permissive); err != nil {
		t.Fatalf("the same session must still reach a workspace that does not enforce SSO, got %v", err)
	}
}

func TestAnSSOSessionIsAcceptedByAnSSOEnforcingWorkspace(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	session := passwordSession(actorID)
	session.AuthMethod = entity.SessionAuthMethodSSO

	h.expectActorIsMember(workspaceID, actorID)
	h.expectAuthEnforcement(workspaceID, entity.AuthEnforcementSSO)
	h.authorizer.EXPECT().
		Authorize(gomock.Any(), entity.MembershipRoleAdmin, entity.ResourceMembership, entity.ActionRead).
		Return(nil)
	h.memberships.EXPECT().ListByWorkspaceID(gomock.Any(), workspaceID).Return(nil, nil)

	ctx := identity.WithSession(context.Background(), session)

	if _, err := h.service.ListMembers(ctx, workspaceID); err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
}

func TestNarrowingIsNotAppliedToTheAuthPolicyItself(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorIsMember(workspaceID, actorID)
	h.authorizer.EXPECT().
		Authorize(gomock.Any(), entity.MembershipRoleAdmin, entity.ResourceWorkspace, entity.ActionUpdate).
		Return(nil)
	h.authPolicies.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, policy entity.WorkspaceAuthPolicy) (entity.WorkspaceAuthPolicy, error) {
			return policy, nil
		})

	policy, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcementSSO)
	if err != nil {
		t.Fatalf("an admin on a password session must still be able to set the policy, got %v", err)
	}

	if policy.Enforcement != entity.AuthEnforcementSSO {
		t.Fatalf("enforcement = %q, want sso", policy.Enforcement)
	}
}

func TestSetAuthPolicyRejectsAnUnknownEnforcement(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorIsMember(workspaceID, actorID)
	h.authorizer.EXPECT().
		Authorize(gomock.Any(), entity.MembershipRoleAdmin, entity.ResourceWorkspace, entity.ActionUpdate).
		Return(nil)

	_, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcement("mandatory"))

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("SetAuthPolicy error = %v, want a ValidationError", err)
	}
}

func TestARequestWithoutASessionIsRefusedByTheNarrowingCheck(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorIsMember(workspaceID, actorID)

	_, err := h.service.ListMembers(identity.Into(context.Background(), actorID), workspaceID)
	if !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf("ListMembers error = %v, want ErrAccountForbidden", err)
	}
}

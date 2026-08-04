package workspace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestSetAuthPolicyRejectsAnUnknownEnforcement(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayActOn(
		workspaceID,
		actorID,
		entity.ResourceAuthPolicy,
		entity.ActionUpdate,
		workspaceWithStatus(workspaceID, entity.WorkspaceStatusActive),
	)

	_, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcement("mandatory"))

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("SetAuthPolicy error = %v, want a ValidationError", err)
	}
}

func TestTheAuthPolicyScreenIsReachedThroughItsOwnResource(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayActOn(
		workspaceID,
		actorID,
		entity.ResourceAuthPolicy,
		entity.ActionUpdate,
		workspaceWithStatus(workspaceID, entity.WorkspaceStatusActive),
	)

	h.authPolicies.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, policy entity.WorkspaceAuthPolicy) (entity.WorkspaceAuthPolicy, error) {
			return policy, nil
		})

	policy, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcementSSO)
	if err != nil {
		t.Fatalf("SetAuthPolicy: %v", err)
	}

	if policy.Enforcement != entity.AuthEnforcementSSO {
		t.Fatalf("enforcement = %q, want sso", policy.Enforcement)
	}
}

func TestListMembersIsRefusedWhenTheDecisionIsRefused(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectDecisionRefused(
		workspaceID,
		entity.ResourceMembership,
		entity.ActionRead,
		entity.ErrWorkspaceAuthMethodNotPermitted,
	)

	_, err := h.service.ListMembers(actingAs(actorID), workspaceID, service.ListMembersInput{})
	if !errors.Is(err, entity.ErrWorkspaceAuthMethodNotPermitted) {
		t.Fatalf("ListMembers error = %v, want ErrWorkspaceAuthMethodNotPermitted", err)
	}
}

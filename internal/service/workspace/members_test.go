package workspace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestChangingYourOwnRoleIsRefused(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)

	_, err := h.service.ChangeMemberRole(
		actingAs(actorID),
		workspaceID,
		actorID,
		entity.MembershipRoleMember,
	)
	if !errors.Is(err, entity.ErrMembershipSelfRoleChange) {
		t.Fatalf("ChangeMemberRole error = %v, want ErrMembershipSelfRoleChange", err)
	}
}

func TestADirectoryManagedMembershipRejectsARoleChange(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceDirectory)

	_, err := h.service.ChangeMemberRole(
		actingAs(actorID),
		workspaceID,
		memberID,
		entity.MembershipRoleAdmin,
	)
	if !errors.Is(err, entity.ErrMembershipDirectoryManaged) {
		t.Fatalf("ChangeMemberRole error = %v, want ErrMembershipDirectoryManaged", err)
	}
}

func TestADirectoryManagedMembershipRejectsRemoval(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceDirectory)

	err := h.service.RemoveMember(actingAs(actorID), workspaceID, memberID, nil)
	if !errors.Is(err, entity.ErrMembershipDirectoryManaged) {
		t.Fatalf("RemoveMember error = %v, want ErrMembershipDirectoryManaged", err)
	}
}

func TestAViewerIsAnAcceptedRole(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectAccount(entity.Account{
		ID:          memberID,
		Status:      entity.AccountStatusActive,
		DisplayName: "Rae Okafor",
		Email:       "rae@meridian.co",
	})

	var captured entity.Membership

	h.memberships.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, membership entity.Membership) (entity.Membership, error) {
			captured = membership

			return membership, nil
		})

	if _, err := h.service.AddMember(
		actingAs(actorID),
		workspaceID,
		memberID,
		entity.MembershipRoleViewer,
	); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if captured.Role != entity.MembershipRoleViewer {
		t.Fatalf("stored role = %q, want viewer", captured.Role)
	}

	if captured.Source != entity.MembershipSourceManual {
		t.Fatalf("stored source = %q, want a membership created by hand to say so", captured.Source)
	}
}

func TestRemovingAMemberRefusesAReassignmentTargetOutsideTheWorkspace(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	outsiderID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceManual)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), memberID).Return(nil, nil)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, outsiderID).
		Return(entity.Membership{}, entity.ErrMembershipNotFound)

	err := h.service.RemoveMember(actingAs(actorID), workspaceID, memberID, &outsiderID)

	assertReassignRejected(t, err)
}

func TestRemovingAMemberRefusesReassigningToThemselves(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceManual)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), memberID).Return(nil, nil)

	err := h.service.RemoveMember(actingAs(actorID), workspaceID, memberID, &memberID)

	assertReassignRejected(t, err)
}

func TestRemovingAMemberRefusesADeactivatedReassignmentTarget(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	targetID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceManual)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), memberID).Return(nil, nil)
	h.expectMembership(workspaceID, targetID, entity.MembershipRoleMember, entity.MembershipSourceManual)
	h.expectAccount(entity.Account{ID: targetID, Status: entity.AccountStatusDeactivated})

	err := h.service.RemoveMember(actingAs(actorID), workspaceID, memberID, &targetID)

	assertReassignRejected(t, err)
}

func TestRemovingAMemberAcceptsAnActiveMemberAsTheTarget(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	targetID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceManual)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), memberID).Return(nil, nil)
	h.expectMembership(workspaceID, targetID, entity.MembershipRoleMember, entity.MembershipSourceManual)
	h.expectAccount(entity.Account{ID: targetID, Status: entity.AccountStatusActive})
	h.memberships.EXPECT().Delete(gomock.Any(), workspaceID, memberID).Return(nil)

	if err := h.service.RemoveMember(actingAs(actorID), workspaceID, memberID, &targetID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
}

func TestTheRemovalPreviewNamesTheTeamsTheMemberWouldLeave(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	left := []entity.Team{
		{ID: uuid.New(), WorkspaceID: workspaceID, Key: "MOB", Name: "Mobile"},
		{ID: uuid.New(), WorkspaceID: workspaceID, Key: "PLT", Name: "Data Platform"},
	}

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceManual)
	h.expectAccount(entity.Account{ID: memberID, DisplayName: "Rae Okafor", Email: "rae@meridian.co"})
	h.teams.EXPECT().ListByWorkspaceMember(gomock.Any(), workspaceID, memberID).Return(left, nil)
	h.memberships.EXPECT().ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), memberID).Return(nil, nil)

	preview, err := h.service.PreviewMemberRemoval(actingAs(actorID), workspaceID, memberID)
	if err != nil {
		t.Fatalf("PreviewMemberRemoval: %v", err)
	}

	if len(preview.Teams) != 2 || preview.Teams[0].Key != "MOB" {
		t.Fatalf("preview teams = %+v, want the teams the member would be removed from", preview.Teams)
	}

	if preview.Member.DisplayName != "Rae Okafor" {
		t.Fatalf("preview member = %+v, want it named", preview.Member)
	}

	if preview.SoleAdmin || preview.DirectoryManaged {
		t.Fatalf("preview flags = %+v, want an ordinary member reported as removable", preview)
	}
}

func TestTheRemovalPreviewFlagsASoleAdministratorWithoutRefusing(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	adminID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, adminID, entity.MembershipRoleAdmin, entity.MembershipSourceManual)
	h.expectAccount(entity.Account{ID: adminID, DisplayName: "Ada Admin", Email: "ada@meridian.co"})
	h.teams.EXPECT().ListByWorkspaceMember(gomock.Any(), workspaceID, adminID).Return(nil, nil)
	h.memberships.EXPECT().
		ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), adminID).
		Return([]uuid.UUID{workspaceID}, nil)

	preview, err := h.service.PreviewMemberRemoval(actingAs(actorID), workspaceID, adminID)
	if err != nil {
		t.Fatalf("the preview must report the obstacle, not raise it: %v", err)
	}

	if !preview.SoleAdmin {
		t.Fatal("preview did not flag the sole administrator")
	}
}

func TestTheRemovalPreviewAndTheRemovalAgreeOnASoleAdministrator(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	adminID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, adminID, entity.MembershipRoleAdmin, entity.MembershipSourceManual)
	h.expectAccount(entity.Account{ID: adminID, DisplayName: "Ada Admin"})
	h.teams.EXPECT().ListByWorkspaceMember(gomock.Any(), workspaceID, adminID).Return(nil, nil)
	h.memberships.EXPECT().
		ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), adminID).
		Return([]uuid.UUID{workspaceID}, nil)

	preview, err := h.service.PreviewMemberRemoval(actingAs(actorID), workspaceID, adminID)
	if err != nil {
		t.Fatalf("PreviewMemberRemoval: %v", err)
	}

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, adminID, entity.MembershipRoleAdmin, entity.MembershipSourceManual)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().
		ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), adminID).
		Return([]uuid.UUID{workspaceID}, nil)

	err = h.service.RemoveMember(actingAs(actorID), workspaceID, adminID, nil)

	if !preview.SoleAdmin || !errors.Is(err, entity.ErrAccountLastWorkspaceAdmin) {
		t.Fatalf("preview soleAdmin = %t and removal error = %v, want the preview to predict the refusal", preview.SoleAdmin, err)
	}
}

func assertReassignRejected(t *testing.T, err error) {
	t.Helper()

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("RemoveMember error = %v, want a ValidationError", err)
	}

	if validation.Fields[0].Field != "reassignTo" {
		t.Fatalf("field = %q, want the error attributed to reassignTo", validation.Fields[0].Field)
	}
}

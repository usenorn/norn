package workspace_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestASyncedMemberIsRefusedAManualEditWhileTheDirectoryIsLicensed(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceDirectory)

	_, err := h.service.ChangeMemberRole(
		actingAs(actorID), workspaceID, memberID, entity.MembershipRoleAdmin,
	)
	if !errors.Is(err, entity.ErrMembershipDirectoryManaged) {
		t.Fatalf("ChangeMemberRole error = %v, want ErrMembershipDirectoryManaged", err)
	}
}

func TestALapsedDirectoryLicenceReleasesTheLockRatherThanFreezingTheWorkspace(t *testing.T) {
	h := newHarnessWithLicence(t, entity.Licence{})

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceDirectory)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil).AnyTimes()
	h.memberships.EXPECT().
		UpdateRole(gomock.Any(), workspaceID, memberID, entity.MembershipRoleAdmin).
		Return(entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   memberID,
			Role:        entity.MembershipRoleAdmin,
			Source:      entity.MembershipSourceDirectory,
		}, nil)
	h.expectAccount(entity.Account{
		ID:          memberID,
		Status:      entity.AccountStatusActive,
		DisplayName: "Rae Okafor",
		Email:       "rae@northwind.co",
	})

	_, err := h.service.ChangeMemberRole(
		actingAs(actorID), workspaceID, memberID, entity.MembershipRoleAdmin,
	)
	if err != nil {
		t.Fatalf(
			"ChangeMemberRole error = %v. The lock on a synced member exists only because the "+
				"next sync would overwrite the change; with synchronization unlicensed nothing "+
				"syncs them, so refusing would leave members nobody can administer at all.",
			err,
		)
	}
}

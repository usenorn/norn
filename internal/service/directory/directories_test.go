package directory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestTheDirectoryTakesOverAMembershipSomebodyAddedByHand(t *testing.T) {
	h := newHarness(t, licensed())

	workspaceID := uuid.New()
	accountID := uuid.New()

	h.connection(workspaceID, "")

	h.directories.EXPECT().
		GetUserByName(gomock.Any(), workspaceID, "rae@northwind.co").
		Return(entity.DirectoryUser{}, entity.ErrDirectoryUserNotFound)

	h.accounts.EXPECT().
		GetByEmail(gomock.Any(), "rae@northwind.co").
		Return(entity.Account{ID: accountID, Status: entity.AccountStatusActive}, nil)

	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, accountID).
		Return(entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   accountID,
			Role:        entity.MembershipRoleMember,
			Source:      entity.MembershipSourceManual,
		}, nil)

	h.memberships.EXPECT().
		SetSource(gomock.Any(), workspaceID, accountID, entity.MembershipSourceDirectory).
		Return(entity.Membership{Source: entity.MembershipSourceDirectory}, nil)

	h.directories.EXPECT().
		SaveUser(gomock.Any(), gomock.Any()).
		Return(entity.DirectoryUser{AccountID: accountID, UserName: "rae@northwind.co"}, nil)

	if _, err := h.service.PutUser(context.Background(), workspaceID, nil, service.DirectoryProfile{
		UserName: "rae@northwind.co",
		Active:   true,
	}); err != nil {
		t.Fatalf("PutUser error = %v", err)
	}
}

func TestSyncRefusesToRemoveTheLastAdministrator(t *testing.T) {
	h := newHarness(t, licensed())

	workspaceID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()

	h.connection(workspaceID, "")
	h.expectNoWorkload(workspaceID)

	h.directories.EXPECT().
		GetUser(gomock.Any(), workspaceID, userID).
		Return(entity.DirectoryUser{
			ID:          userID,
			WorkspaceID: workspaceID,
			AccountID:   accountID,
			UserName:    "rae@northwind.co",
		}, nil)

	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, accountID).
		Return(entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   accountID,
			Role:        entity.MembershipRoleAdmin,
			Source:      entity.MembershipSourceDirectory,
		}, nil)

	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().
		ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), accountID).
		Return([]uuid.UUID{workspaceID}, nil)

	_, err := h.service.DeleteUser(context.Background(), workspaceID, userID)

	var lastAdmin entity.LastWorkspaceAdminError

	if !errors.As(err, &lastAdmin) {
		t.Fatalf("DeleteUser error = %v, want LastWorkspaceAdminError", err)
	}
}

func TestDeprovisioningDeactivatesRatherThanRemoving(t *testing.T) {
	h := newHarness(t, licensed())

	workspaceID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()

	h.connection(workspaceID, "")
	h.expectNoWorkload(workspaceID)

	h.directories.EXPECT().
		GetUser(gomock.Any(), workspaceID, userID).
		Return(entity.DirectoryUser{
			ID: userID, WorkspaceID: workspaceID, AccountID: accountID, UserName: "rae@northwind.co",
		}, nil)

	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, accountID).
		Return(entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   accountID,
			Role:        entity.MembershipRoleMember,
			Source:      entity.MembershipSourceDirectory,
		}, nil)

	var stoppedAt *time.Time

	h.memberships.EXPECT().
		SetDeactivated(gomock.Any(), workspaceID, accountID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, at *time.Time) (entity.Membership, error) {
			stoppedAt = at

			return entity.Membership{DeactivatedAt: at}, nil
		})

	h.directories.EXPECT().SaveUser(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, user entity.DirectoryUser) (entity.DirectoryUser, error) {
			if user.Active {
				t.Error("the directory user was left active after deprovisioning")
			}

			return user, nil
		})

	if _, err := h.service.DeleteUser(context.Background(), workspaceID, userID); err != nil {
		t.Fatalf("DeleteUser error = %v", err)
	}

	if stoppedAt == nil {
		t.Fatal(
			"deprovisioning did not deactivate the membership. Removing the row would take the " +
				"person out of the members list along with any sight of the work they were holding.",
		)
	}
}

func TestAnUnlicensedInstanceRefusesEveryDirectoryOperation(t *testing.T) {
	h := newHarness(t, entity.Licence{})

	workspaceID := uuid.New()

	if available := h.service.Availability(context.Background()); available.Available {
		t.Error("an unlicensed instance reports directory synchronization as available")
	}

	if _, err := h.service.Authenticate(context.Background(), "nrnscim_anything"); !errors.Is(
		err, entity.ErrDirectoryUnlicensed,
	) {
		t.Errorf("Authenticate error = %v, want ErrDirectoryUnlicensed", err)
	}

	if _, err := h.service.PutUser(
		context.Background(), workspaceID, nil, service.DirectoryProfile{UserName: "rae@northwind.co"},
	); !errors.Is(err, entity.ErrDirectoryUnlicensed) {
		t.Errorf("PutUser error = %v, want ErrDirectoryUnlicensed", err)
	}

	if _, err := h.service.DeleteUser(
		context.Background(), workspaceID, uuid.New(),
	); !errors.Is(err, entity.ErrDirectoryUnlicensed) {
		t.Errorf("DeleteUser error = %v, want ErrDirectoryUnlicensed", err)
	}
}

func TestAGroupMatchingNoTeamIsRecordedRatherThanDroppedSilently(t *testing.T) {
	h := newHarness(t, licensed())

	workspaceID := uuid.New()

	h.connection(workspaceID, "")

	h.teams.EXPECT().
		ListVisibleTo(gomock.Any(), workspaceID, gomock.Any(), entity.TeamStatusActive, true).
		Return([]entity.Team{{ID: uuid.New(), Key: "ENG", Name: "Engineering"}}, nil)

	h.directories.EXPECT().SaveGroup(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, group entity.DirectoryGroup) (entity.DirectoryGroup, error) {
			if group.Mapped() {
				t.Error("a group with no matching team was mapped to one anyway")
			}

			return group, nil
		})

	h.directories.EXPECT().ListGroupMembers(gomock.Any(), gomock.Any()).Return(nil, nil)

	if _, err := h.service.PutGroup(context.Background(), workspaceID, nil, service.DirectoryGroupProfile{
		DisplayName: "marketing-team",
	}); err != nil {
		t.Fatalf("PutGroup error = %v", err)
	}

	skipped := false

	for _, change := range h.changes() {
		if change.Kind == entity.DirectoryGroupSkipped {
			skipped = true
		}
	}

	if !skipped {
		t.Fatal(
			"an unmapped group left no trace in the sync log. Silently ignoring a group is how " +
				"an administrator ends up believing team membership is being managed when it is not.",
		)
	}
}

func TestTheAdminGroupDecidesTheRole(t *testing.T) {
	connection := entity.DirectoryConnection{AdminGroup: "norn-admins"}

	if role := connection.RoleFor([]string{"engineering", "norn-admins"}); role != entity.MembershipRoleAdmin {
		t.Errorf("role for a member of the admin group = %q, want admin", role)
	}

	if role := connection.RoleFor([]string{"engineering"}); role != entity.MembershipRoleMember {
		t.Errorf("role outside the admin group = %q, want member", role)
	}

	none := entity.DirectoryConnection{}

	if role := none.RoleFor([]string{"norn-admins"}); role != entity.MembershipRoleMember {
		t.Errorf(
			"role %q with no admin group configured, want member: a directory that names no "+
				"admin group must never hand out administration",
			role,
		)
	}
}

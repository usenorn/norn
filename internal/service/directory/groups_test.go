package directory_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type roster struct {
	workspaceID uuid.UUID
	teamID      uuid.UUID
	groupID     uuid.UUID
	directoryID uuid.UUID
	accountID   uuid.UUID
}

func (h *harness) mappedGroup(t *testing.T) roster {
	t.Helper()

	r := roster{
		workspaceID: uuid.New(),
		teamID:      uuid.New(),
		groupID:     uuid.New(),
		directoryID: uuid.New(),
		accountID:   uuid.New(),
	}

	h.connection(r.workspaceID, "")

	h.directories.EXPECT().
		GetGroup(gomock.Any(), r.workspaceID, r.groupID).
		Return(entity.DirectoryGroup{
			ID:          r.groupID,
			WorkspaceID: r.workspaceID,
			DisplayName: "engineering",
			TeamID:      &r.teamID,
		}, nil)

	h.directories.EXPECT().
		GetUser(gomock.Any(), r.workspaceID, r.directoryID).
		Return(entity.DirectoryUser{
			ID:          r.directoryID,
			WorkspaceID: r.workspaceID,
			AccountID:   r.accountID,
			UserName:    "rae@northwind.co",
			Active:      true,
		}, nil)

	return r
}

func (h *harness) emissionsOf(event entity.WebhookEvent) []entity.WebhookOutboxEntry {
	var matching []entity.WebhookOutboxEntry

	for _, entry := range h.emitted {
		if entry.Event == event {
			matching = append(matching, entry)
		}
	}

	return matching
}

func TestJoiningAMappedGroupTellsTheTeamRosterWatchers(t *testing.T) {
	h := newHarness(t, licensed())
	r := h.mappedGroup(t)

	h.directories.EXPECT().AddGroupMember(gomock.Any(), r.groupID, r.directoryID).Return(nil)
	h.teamMembers.EXPECT().
		Get(gomock.Any(), r.teamID, r.accountID).
		Return(entity.TeamMembership{}, entity.ErrTeamMembershipNotFound)
	h.teamMembers.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(entity.TeamMembership{TeamID: r.teamID, AccountID: r.accountID}, nil)
	h.teams.EXPECT().
		GetByID(gomock.Any(), r.teamID).
		Return(entity.Team{ID: r.teamID, WorkspaceID: r.workspaceID, Key: "ENG", Name: "Engineering"}, nil)

	if _, err := h.service.EditGroupMembers(
		context.Background(), r.workspaceID, r.groupID,
		service.DirectoryMembershipEdit{Add: []uuid.UUID{r.directoryID}},
	); err != nil {
		t.Fatalf("EditGroupMembers: %v", err)
	}

	added := h.emissionsOf(entity.WebhookTeamMembershipAdded)
	if len(added) != 1 {
		t.Fatalf(
			"the provider put somebody on a team and %d team_membership.added events left the "+
				"workspace. A roster that only announces the changes a human made leaves an "+
				"integration blind to exactly the ones nobody is watching a screen for.",
			len(added),
		)
	}

	entry := added[0]

	if entry.TeamID != r.teamID {
		t.Fatalf(
			"the event carries team %s, want the team the group maps to (%s). team_membership.* "+
				"is delivered only to admins whose scope covers this team id, so the wrong one "+
				"either hides the event or hands it to people who cannot see that team.",
			entry.TeamID, r.teamID,
		)
	}

	if entry.SubjectID != r.accountID {
		t.Errorf("the event names subject %s, want the account that joined (%s)", entry.SubjectID, r.accountID)
	}

	if entry.SubjectKind != string(entity.ResourceTeamMembership) {
		t.Errorf("the event subject kind is %q, want %q", entry.SubjectKind, entity.ResourceTeamMembership)
	}

	if entry.WorkspaceID != r.workspaceID {
		t.Errorf("the event carries workspace %s, want %s", entry.WorkspaceID, r.workspaceID)
	}
}

func TestLeavingAMappedGroupTellsTheTeamRosterWatchers(t *testing.T) {
	h := newHarness(t, licensed())
	r := h.mappedGroup(t)

	h.directories.EXPECT().RemoveGroupMember(gomock.Any(), r.groupID, r.directoryID).Return(nil)
	h.teamMembers.EXPECT().Delete(gomock.Any(), r.teamID, r.accountID).Return(nil)
	h.teams.EXPECT().
		GetByID(gomock.Any(), r.teamID).
		Return(entity.Team{ID: r.teamID, WorkspaceID: r.workspaceID, Key: "ENG", Name: "Engineering"}, nil)

	if _, err := h.service.EditGroupMembers(
		context.Background(), r.workspaceID, r.groupID,
		service.DirectoryMembershipEdit{Remove: []uuid.UUID{r.directoryID}},
	); err != nil {
		t.Fatalf("EditGroupMembers: %v", err)
	}

	removed := h.emissionsOf(entity.WebhookTeamMembershipRemoved)
	if len(removed) != 1 {
		t.Fatalf(
			"the provider took somebody off a team and %d team_membership.removed events left "+
				"the workspace. An integration that never hears the removal keeps granting the "+
				"access the directory just withdrew.",
			len(removed),
		)
	}

	if removed[0].TeamID != r.teamID || removed[0].SubjectID != r.accountID {
		t.Fatalf(
			"the removal names team %s and subject %s, want team %s and account %s",
			removed[0].TeamID, removed[0].SubjectID, r.teamID, r.accountID,
		)
	}
}

func TestAGroupWhoseTeamIsGoneStillSyncsQuietly(t *testing.T) {
	h := newHarness(t, licensed())
	r := h.mappedGroup(t)

	h.directories.EXPECT().AddGroupMember(gomock.Any(), r.groupID, r.directoryID).Return(nil)
	h.teamMembers.EXPECT().
		Get(gomock.Any(), r.teamID, r.accountID).
		Return(entity.TeamMembership{}, entity.ErrTeamMembershipNotFound)
	h.teamMembers.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(entity.TeamMembership{TeamID: r.teamID, AccountID: r.accountID}, nil)
	h.teams.EXPECT().
		GetByID(gomock.Any(), r.teamID).
		Return(entity.Team{}, entity.ErrTeamNotFound)

	if _, err := h.service.EditGroupMembers(
		context.Background(), r.workspaceID, r.groupID,
		service.DirectoryMembershipEdit{Add: []uuid.UUID{r.directoryID}},
	); err != nil {
		t.Fatalf(
			"a group pointing at a team that no longer exists failed the whole sync: %v\nOne "+
				"stale mapping must not stop the provider from reconciling everybody else.",
			err,
		)
	}

	if len(h.emitted) != 0 {
		t.Fatalf(
			"%d events left the workspace for a team that could not be read. Without the team "+
				"there is no team id to scope the delivery by, and an unscoped roster event "+
				"reaches admins who cannot see that team.",
			len(h.emitted),
		)
	}
}

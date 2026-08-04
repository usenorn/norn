package team_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func TestArchivingATeamKeepsItReadableAndKeepsItsKeyReserved(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)
	h.expectTeam(publicTeam(workspaceID, teamID))
	h.teams.EXPECT().
		Archive(gomock.Any(), teamID, gomock.Any()).
		Return(archivedTeam(workspaceID, teamID), nil)

	archived, err := h.service.Archive(actingAs(actorID), workspaceID, teamID)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}

	if archived.Key != "MOB" {
		t.Fatalf("key = %q, want an archived team to keep the key its issues quote", archived.Key)
	}

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionRead)
	h.expectTeam(archivedTeam(workspaceID, teamID))

	readable, err := h.service.Get(actingAs(actorID), workspaceID, teamID)
	if err != nil {
		t.Fatalf("an archived team must stay readable, got %v", err)
	}

	if readable.Key != "MOB" {
		t.Fatalf("key = %q, want the reference still resolvable", readable.Key)
	}

	surface := reflect.TypeOf((*repository.Team)(nil)).Elem()
	for i := range surface.NumMethod() {
		if strings.Contains(strings.ToLower(surface.Method(i).Name), "delete") {
			t.Errorf("repository.Team exposes %q, which would free a key and break old references", surface.Method(i).Name)
		}
	}
}

func TestArchivingTwiceIsRefused(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)
	h.expectTeam(archivedTeam(workspaceID, teamID))
	h.teams.EXPECT().
		Archive(gomock.Any(), teamID, gomock.Any()).
		Return(entity.Team{}, entity.ErrTeamArchived)

	_, err := h.service.Archive(actingAs(actorID), workspaceID, teamID)
	if !errors.Is(err, entity.ErrTeamArchived) {
		t.Fatalf("Archive error = %v, want ErrTeamArchived", err)
	}
}

func TestUnarchivingBringsAnArchivedTeamBack(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)
	h.expectTeam(archivedTeam(workspaceID, teamID))
	h.teams.EXPECT().Unarchive(gomock.Any(), teamID).Return(publicTeam(workspaceID, teamID), nil)

	restored, err := h.service.Unarchive(actingAs(actorID), workspaceID, teamID)
	if err != nil {
		t.Fatalf("Unarchive: %v", err)
	}

	if restored.Archived() {
		t.Fatal("Unarchive returned a team still marked archived")
	}
}

func TestUnarchivingALiveTeamIsRefused(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)
	h.expectTeam(publicTeam(workspaceID, teamID))
	h.teams.EXPECT().Unarchive(gomock.Any(), teamID).Return(entity.Team{}, entity.ErrTeamNotArchived)

	_, err := h.service.Unarchive(actingAs(actorID), workspaceID, teamID)
	if !errors.Is(err, entity.ErrTeamNotArchived) {
		t.Fatalf("Unarchive error = %v, want ErrTeamNotArchived", err)
	}
}

func TestAnArchivedTeamRefusesRenamesAndMembershipChanges(t *testing.T) {
	name := "Mobile Apps"

	cases := map[string]func(h *harness, workspaceID, teamID, actorID uuid.UUID) error{
		"rename": func(h *harness, workspaceID, teamID, actorID uuid.UUID) error {
			h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)
			h.expectTeam(archivedTeam(workspaceID, teamID))

			_, err := h.service.Update(actingAs(actorID), workspaceID, teamID, service.UpdateTeamInput{Name: &name})

			return err
		},
		"add a member": func(h *harness, workspaceID, teamID, actorID uuid.UUID) error {
			h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeamMembership, entity.ActionManage)
			h.expectTeam(archivedTeam(workspaceID, teamID))

			_, err := h.service.AddMember(actingAs(actorID), workspaceID, teamID, uuid.New())

			return err
		},
		"remove a member": func(h *harness, workspaceID, teamID, actorID uuid.UUID) error {
			h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeamMembership, entity.ActionManage)
			h.expectTeam(archivedTeam(workspaceID, teamID))

			return h.service.RemoveMember(actingAs(actorID), workspaceID, teamID, uuid.New())
		},
	}

	for name, attempt := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			if err := attempt(h, uuid.New(), uuid.New(), uuid.New()); !errors.Is(err, entity.ErrTeamArchived) {
				t.Fatalf("%s on an archived team = %v, want ErrTeamArchived", name, err)
			}
		})
	}
}

func TestAnArchivedTeamStillListsItsMembers(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()
	memberID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeamMembership, entity.ActionRead)
	h.expectTeam(archivedTeam(workspaceID, teamID))
	h.teamMembers.EXPECT().
		ListByTeamID(gomock.Any(), teamID).
		Return([]entity.TeamMembership{{TeamID: teamID, AccountID: memberID}}, nil)
	h.accounts.EXPECT().
		ListByIDs(gomock.Any(), []uuid.UUID{memberID}).
		Return([]entity.Account{{ID: memberID, DisplayName: "Rae Okafor", Email: "rae@northwind.co"}}, nil)

	members, err := h.service.ListMembers(actingAs(actorID), workspaceID, teamID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}

	if len(members) != 1 || members[0].DisplayName != "Rae Okafor" {
		t.Fatalf("members = %+v, want the roster named", members)
	}
}

func TestTeamMutationsAreRefusedWhileTheWorkspaceIsPendingDeletion(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayActOn(
		workspaceID,
		actorID,
		entity.MembershipRoleAdmin,
		entity.ResourceTeam,
		entity.ActionManage,
		entity.WorkspaceStatusPendingDeletion,
	)

	_, err := h.service.Create(actingAs(actorID), service.CreateTeamInput{
		WorkspaceID: workspaceID,
		Key:         "MOB",
		Name:        "Mobile",
	})

	var deleted entity.WorkspaceDeletedError
	if !errors.As(err, &deleted) {
		t.Fatalf("Create error = %v, want a WorkspaceDeletedError", err)
	}

	if deleted.PurgeAfter == nil {
		t.Fatal("the refusal must name the date the workspace stops being recoverable")
	}
}

func TestTeamsStayReadableWhileTheWorkspaceIsPendingDeletion(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayActOn(
		workspaceID,
		actorID,
		entity.MembershipRoleAdmin,
		entity.ResourceTeam,
		entity.ActionRead,
		entity.WorkspaceStatusPendingDeletion,
	)
	h.teams.EXPECT().
		ListVisibleTo(gomock.Any(), workspaceID, actorID, entity.TeamStatus(""), true).
		Return(nil, nil)

	if _, err := h.service.List(actingAs(actorID), workspaceID, ""); err != nil {
		t.Fatalf("a workspace pending deletion must still read, got %v", err)
	}
}

func TestAddMemberIsRefusedForADeactivatedAccount(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()
	memberID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeamMembership, entity.ActionManage)
	h.expectTeam(publicTeam(workspaceID, teamID))
	h.accounts.EXPECT().
		GetByID(gomock.Any(), memberID).
		Return(entity.Account{ID: memberID, Status: entity.AccountStatusDeactivated}, nil)

	_, err := h.service.AddMember(actingAs(actorID), workspaceID, teamID, memberID)
	if !errors.Is(err, entity.ErrAccountDeactivated) {
		t.Fatalf("AddMember error = %v, want ErrAccountDeactivated", err)
	}
}

func TestAddMemberIsRefusedForSomeoneOutsideTheWorkspace(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()
	memberID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeamMembership, entity.ActionManage)
	h.expectTeam(publicTeam(workspaceID, teamID))
	h.accounts.EXPECT().
		GetByID(gomock.Any(), memberID).
		Return(entity.Account{ID: memberID, Status: entity.AccountStatusActive}, nil)
	h.teamMembers.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(entity.TeamMembership{}, entity.ErrMembershipNotFound)

	_, err := h.service.AddMember(actingAs(actorID), workspaceID, teamID, memberID)
	if !errors.Is(err, entity.ErrMembershipNotFound) {
		t.Fatalf("AddMember error = %v, want ErrMembershipNotFound", err)
	}
}

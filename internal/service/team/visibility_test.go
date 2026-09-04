package team_test

import (
	"context"
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

func TestAPrivateTeamIsInvisibleToAWorkspaceMemberWhoIsNotOnIt(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorSees(workspaceID, actorID, entity.MembershipRoleMember, entity.ResourceTeam, entity.ActionRead)
	h.expectTeam(privateTeam(workspaceID, teamID))

	_, err := h.service.Get(actingAs(actorID), workspaceID, teamID)

	if !errors.Is(err, entity.ErrTeamNotFound) {
		t.Fatalf("Get error = %v, want ErrTeamNotFound", err)
	}

	if errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatal("a private team must not answer with a forbidden error, which would confirm it exists")
	}
}

func TestAPrivateTeamIsVisibleToItsOwnMembers(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorSees(workspaceID, actorID, entity.MembershipRoleMember, entity.ResourceTeam, entity.ActionRead, teamID)
	h.expectTeam(privateTeam(workspaceID, teamID))

	team, err := h.service.Get(actingAs(actorID), workspaceID, teamID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if team.ID != teamID {
		t.Fatalf("team id = %v, want %v", team.ID, teamID)
	}
}

func TestAWorkspaceAdministratorSeesAPrivateTeamTheyAreNotOn(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionRead)
	h.expectTeam(privateTeam(workspaceID, teamID))

	team, err := h.service.Get(actingAs(actorID), workspaceID, teamID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if team.ID != teamID {
		t.Fatalf("team id = %v, want %v", team.ID, teamID)
	}
}

func TestAWorkspaceAdministratorAdministersAPrivateTeamTheyAreNotOn(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)
	h.expectTeam(privateTeam(workspaceID, teamID))

	name := "Platform"

	h.teams.EXPECT().
		UpdateSettings(gomock.Any(), teamID, repository.TeamSettings{
			Name:       name,
			IconColor:  entity.DefaultTeamColor,
			Estimation: entity.DefaultTeamEstimation,
			Visibility: entity.TeamVisibilityPrivate,
		}).
		Return(privateTeam(workspaceID, teamID), nil)

	if _, err := h.service.Update(
		actingAs(actorID),
		workspaceID,
		teamID,
		service.UpdateTeamInput{Name: &name},
	); err != nil {
		t.Fatalf("Update: %v", err)
	}
}

func TestListOmitsPrivateTeamsForAnOrdinaryMember(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorSees(workspaceID, actorID, entity.MembershipRoleMember, entity.ResourceTeam, entity.ActionRead)

	var capturedIncludePrivate bool

	h.teams.EXPECT().
		ListVisibleTo(gomock.Any(), workspaceID, actorID, entity.TeamStatus(""), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _ uuid.UUID,
			_ entity.TeamStatus,
			includePrivate bool,
		) ([]entity.Team, error) {
			capturedIncludePrivate = includePrivate

			return nil, nil
		})

	if _, err := h.service.List(actingAs(actorID), workspaceID, ""); err != nil {
		t.Fatalf("List: %v", err)
	}

	if capturedIncludePrivate {
		t.Fatal("an ordinary member's listing must not ask the store for private teams")
	}
}

func TestListIncludesPrivateTeamsForAnAdministrator(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionRead)

	var capturedIncludePrivate bool

	h.teams.EXPECT().
		ListVisibleTo(gomock.Any(), workspaceID, actorID, entity.TeamStatus(""), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _ uuid.UUID,
			_ entity.TeamStatus,
			includePrivate bool,
		) ([]entity.Team, error) {
			capturedIncludePrivate = includePrivate

			return nil, nil
		})

	if _, err := h.service.List(actingAs(actorID), workspaceID, ""); err != nil {
		t.Fatalf("List: %v", err)
	}

	if !capturedIncludePrivate {
		t.Fatal("an administrator's listing must include private teams so they can be administered")
	}
}

func TestListRejectsAnUnknownStatusFilter(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleMember, entity.ResourceTeam, entity.ActionRead)

	_, err := h.service.List(actingAs(actorID), workspaceID, "retired")

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("List error = %v, want a ValidationError", err)
	}
}

func TestATeamFromAnotherWorkspaceIsReportedAsNotFound(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionRead)
	h.expectTeam(publicTeam(uuid.New(), teamID))

	_, err := h.service.Get(actingAs(actorID), workspaceID, teamID)
	if !errors.Is(err, entity.ErrTeamNotFound) {
		t.Fatalf("Get error = %v, want ErrTeamNotFound", err)
	}
}

func TestAScopeThatOmitsATeamHidesItRatherThanForbiddingIt(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	visible := uuid.New()
	hidden := uuid.New()

	h.expectActorSees(workspaceID, actorID, entity.MembershipRoleMember, entity.ResourceTeam, entity.ActionRead, visible)
	h.expectTeam(privateTeam(workspaceID, hidden))

	_, err := h.service.Get(actingAs(actorID), workspaceID, hidden)

	if !errors.Is(err, entity.ErrTeamNotFound) {
		t.Fatalf("Get error = %v, want ErrTeamNotFound", err)
	}

	if errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatal("a team outside the scope must not answer with a forbidden error, which would confirm it exists")
	}
}

func TestNoLayerExposesATeamIssueCount(t *testing.T) {
	forbidden := []string{"count", "total", "issues", "quota", "progress"}

	surfaces := map[string]reflect.Type{
		"repository.Team": reflect.TypeOf((*repository.Team)(nil)).Elem(),
		"service.Teams":   reflect.TypeOf((*service.Teams)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := strings.ToLower(surface.Method(i).Name)

			for _, word := range forbidden {
				if strings.Contains(method, word) {
					t.Errorf("%s exposes %q, which could leak a private team's contents", name, surface.Method(i).Name)
				}
			}
		}
	}

	team := reflect.TypeOf(entity.Team{})
	for i := range team.NumField() {
		field := strings.ToLower(team.Field(i).Name)

		for _, word := range forbidden {
			if strings.Contains(field, word) {
				t.Errorf("entity.Team carries %q, which would travel to non-members", team.Field(i).Name)
			}
		}
	}
}

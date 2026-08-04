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

func TestCreateRejectsAKeyAlreadyUsedInTheWorkspace(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)
	h.teams.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Team{}, entity.ErrTeamKeyTaken)

	_, err := h.service.Create(actingAs(actorID), service.CreateTeamInput{
		WorkspaceID: workspaceID,
		Key:         "MOB",
		Name:        "Mobile",
	})
	if !errors.Is(err, entity.ErrTeamKeyTaken) {
		t.Fatalf("Create error = %v, want ErrTeamKeyTaken to survive the service unwrapped", err)
	}
}

func TestCreateNormalizesTheKeyBeforeStoringIt(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)

	var captured entity.Team

	h.expectStatesSeeded()
	h.teams.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, team entity.Team) (entity.Team, error) {
			captured = team
			team.ID = uuid.New()

			return team, nil
		})

	if _, err := h.service.Create(actingAs(actorID), service.CreateTeamInput{
		WorkspaceID: workspaceID,
		Key:         " mob ",
		Name:        "Mobile",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.Key != "MOB" {
		t.Fatalf("stored key = %q, want it normalized to MOB", captured.Key)
	}
}

func TestCreateDefaultsANewTeamToPublicVisibility(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)

	var captured entity.Team

	h.expectStatesSeeded()
	h.teams.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, team entity.Team) (entity.Team, error) {
			captured = team

			return team, nil
		})

	if _, err := h.service.Create(actingAs(actorID), service.CreateTeamInput{
		WorkspaceID: workspaceID,
		Key:         "MOB",
		Name:        "Mobile",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.Visibility != entity.TeamVisibilityPublic {
		t.Fatalf("visibility = %q, want a team to start visible to the workspace", captured.Visibility)
	}
}

func TestCreateRejectsAMalformedKey(t *testing.T) {
	cases := map[string]string{
		"too short":    "M",
		"too long":     "TOOLONG",
		"punctuated":   "M-B",
		"with a digit": "M0B",
	}

	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			actorID := uuid.New()
			workspaceID := uuid.New()

			h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)

			_, err := h.service.Create(actingAs(actorID), service.CreateTeamInput{
				WorkspaceID: workspaceID,
				Key:         key,
				Name:        "Mobile",
			})

			var validation entity.ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("Create(%q) error = %v, want a ValidationError", key, err)
			}

			if validation.Fields[0].Field != "key" {
				t.Fatalf("field = %q, want the error attributed to key", validation.Fields[0].Field)
			}
		})
	}
}

func TestCreateRejectsAnUnknownVisibility(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)

	_, err := h.service.Create(actingAs(actorID), service.CreateTeamInput{
		WorkspaceID: workspaceID,
		Key:         "MOB",
		Name:        "Mobile",
		Visibility:  "secret",
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create error = %v, want a ValidationError", err)
	}
}

func TestCreateIsRefusedForAnOrdinaryMember(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectDecisionRefused(workspaceID, entity.ResourceTeam, entity.ActionManage, entity.ErrAccountForbidden)

	_, err := h.service.Create(actingAs(actorID), service.CreateTeamInput{
		WorkspaceID: workspaceID,
		Key:         "MOB",
		Name:        "Mobile",
	})
	if !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf("Create error = %v, want ErrAccountForbidden", err)
	}
}

func TestCreateIsRefusedForANonMemberActor(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectDecisionRefused(workspaceID, entity.ResourceTeam, entity.ActionManage, entity.ErrAccountForbidden)

	_, err := h.service.Create(actingAs(actorID), service.CreateTeamInput{
		WorkspaceID: workspaceID,
		Key:         "MOB",
		Name:        "Mobile",
	})
	if !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf("Create error = %v, want ErrAccountForbidden", err)
	}
}

func TestUpdateChangesTheNameAndNeverTheKey(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)
	h.expectTeam(publicTeam(workspaceID, teamID))

	var capturedName string

	h.teams.EXPECT().
		UpdateSettings(gomock.Any(), teamID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, name string, visibility entity.TeamVisibility) (entity.Team, error) {
			capturedName = name

			updated := publicTeam(workspaceID, id)
			updated.Name = name
			updated.Visibility = visibility

			return updated, nil
		})

	name := "Mobile Apps"

	updated, err := h.service.Update(actingAs(actorID), workspaceID, teamID, service.UpdateTeamInput{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if capturedName != name {
		t.Fatalf("wrote name = %q, want %q", capturedName, name)
	}

	if updated.Key != "MOB" {
		t.Errorf("key = %q, want it untouched by an update", updated.Key)
	}
}

func TestNoLayerAcceptsAKeyChangeAfterCreation(t *testing.T) {
	mutations := []string{"set", "update", "change", "rename"}

	surfaces := map[string]reflect.Type{
		"repository.Team": reflect.TypeOf((*repository.Team)(nil)).Elem(),
		"service.Teams":   reflect.TypeOf((*service.Teams)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := strings.ToLower(surface.Method(i).Name)

			for _, mutation := range mutations {
				if strings.Contains(method, mutation) && strings.Contains(method, "key") {
					t.Errorf("%s exposes %q, which would let an issue reference be rewritten", name, surface.Method(i).Name)
				}
			}
		}
	}

	input := reflect.TypeOf(service.UpdateTeamInput{})
	for i := range input.NumField() {
		if strings.Contains(strings.ToLower(input.Field(i).Name), "key") {
			t.Errorf("service.UpdateTeamInput carries %q, so a key could be submitted for update", input.Field(i).Name)
		}
	}
}

func TestANewTeamIsImmediatelyUsable(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.MembershipRoleAdmin, entity.ResourceTeam, entity.ActionManage)

	var seeded []entity.WorkflowState

	h.states.EXPECT().
		CreateMany(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, states []entity.WorkflowState) ([]entity.WorkflowState, error) {
			seeded = states

			return states, nil
		})

	var created entity.Team

	h.teams.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, team entity.Team) (entity.Team, error) {
			team.ID = uuid.New()
			created = team

			return team, nil
		})

	if _, err := h.service.Create(actingAs(actorID), service.CreateTeamInput{
		WorkspaceID: workspaceID,
		Key:         "MOB",
		Name:        "Mobile",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(seeded) == 0 {
		t.Fatal("a new team was seeded with no workflow states, so it is not usable without configuration")
	}

	present := make(map[entity.StateCategory]bool, len(seeded))

	var defaults, completions int

	for _, state := range seeded {
		present[state.Category] = true

		if state.TeamID != created.ID {
			t.Fatalf("state %q belongs to team %v, want the team just created %v", state.Name, state.TeamID, created.ID)
		}

		if state.IsDefault {
			defaults++
		}

		if state.IsCompletion {
			completions++
		}
	}

	for _, category := range entity.StateCategories() {
		if !present[category] {
			t.Errorf("the seeded set has no %q state, so the system has nowhere to put such an issue", category)
		}
	}

	if defaults != 1 || completions != 1 {
		t.Fatalf("the seeded set has %d defaults and %d completion states, want exactly one of each", defaults, completions)
	}
}

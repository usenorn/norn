package workspace_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) expectActorMayAct(workspaceID, actorID uuid.UUID, action entity.Action, workspace entity.Workspace) {
	h.expectActorMayActOn(workspaceID, actorID, entity.ResourceWorkspace, action, workspace)
}

func (h *harness) expectActorMayActOn(
	workspaceID, actorID uuid.UUID,
	resource entity.Resource,
	action entity.Action,
	workspace entity.Workspace,
) {
	if workspace.Deleted() && action.RequiresLiveWorkspace() {
		h.authorizer.EXPECT().
			Decide(gomock.Any(), matchRequest(workspaceID, resource, action)).
			Return(entity.Decision{}, entity.AccessDeniedError{
				Reason:      entity.DenyReasonWorkspaceDeleted,
				Resource:    resource,
				Action:      action,
				WorkspaceID: workspaceID,
				PurgeAfter:  workspace.PurgeAfter,
			})

		return
	}

	h.authorizer.EXPECT().
		Decide(gomock.Any(), matchRequest(workspaceID, resource, action)).
		Return(entity.Decision{
			Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: actorID},
			Role:      entity.MembershipRoleAdmin,
			Workspace: workspace,
			Scope:     entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true},
		}, nil)
}

func (h *harness) expectDecisionRefused(
	workspaceID uuid.UUID,
	resource entity.Resource,
	action entity.Action,
	err error,
) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), matchRequest(workspaceID, resource, action)).
		Return(entity.Decision{}, err)
}

func matchRequest(workspaceID uuid.UUID, resource entity.Resource, action entity.Action) gomock.Matcher {
	return gomock.Cond(func(request entity.AccessRequest) bool {
		return request.WorkspaceID == workspaceID &&
			request.Resource == resource &&
			request.Action == action
	})
}

func pendingWorkspace(id uuid.UUID, requestedAt time.Time) entity.Workspace {
	purgeAfter := requestedAt.Add(deletionGracePeriod)

	pending := activeWorkspace(id)
	pending.Status = entity.WorkspaceStatusPendingDeletion
	pending.DeletionRequestedAt = &requestedAt
	pending.PurgeAfter = &purgeAfter

	return pending
}

func activeWorkspace(id uuid.UUID) entity.Workspace {
	return entity.Workspace{
		ID:       id,
		Slug:     "northwind",
		Name:     "Northwind",
		Status:   entity.WorkspaceStatusActive,
		Timezone: "Europe/London",
	}
}

func TestUpdateChangesTheNameAndTimezoneAndNeverTheIdentifier(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))

	var capturedName, capturedTimezone string

	h.workspaces.EXPECT().
		UpdateSettings(gomock.Any(), workspaceID, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, name, timezone string, _ *uuid.UUID) (entity.Workspace, error) {
			capturedName = name
			capturedTimezone = timezone

			updated := activeWorkspace(id)
			updated.Name = name
			updated.Timezone = timezone

			return updated, nil
		})

	name := "Northwind Trading"
	timezone := "America/New_York"

	updated, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{
		Name:     &name,
		Timezone: &timezone,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if capturedName != name || capturedTimezone != timezone {
		t.Fatalf("wrote name=%q timezone=%q, want %q and %q", capturedName, capturedTimezone, name, timezone)
	}

	if updated.Slug != "northwind" {
		t.Errorf("slug = %q, want it untouched by an update", updated.Slug)
	}
}

func TestUpdateLeavesUnsuppliedFieldsAlone(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))

	var capturedName, capturedTimezone string

	h.workspaces.EXPECT().
		UpdateSettings(gomock.Any(), workspaceID, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, name, timezone string, _ *uuid.UUID) (entity.Workspace, error) {
			capturedName = name
			capturedTimezone = timezone

			return activeWorkspace(id), nil
		})

	name := "Northwind Trading"

	if _, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{
		Name: &name,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if capturedName != name {
		t.Errorf("name = %q, want %q", capturedName, name)
	}

	if capturedTimezone != "Europe/London" {
		t.Errorf("timezone = %q, want the existing value preserved", capturedTimezone)
	}
}

func TestUpdateRejectsAnUnknownTimezone(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))

	timezone := "Mars/Olympus"

	_, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{
		Timezone: &timezone,
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Update error = %v, want a ValidationError", err)
	}

	if len(validation.Fields) != 1 || validation.Fields[0].Code != entity.ValidationCodeUnknownTimezone {
		t.Fatalf("validation fields = %v, want an unknown_timezone code", validation.Fields)
	}
}

func TestDeleteEntersARecoverableStateAndSchedulesThePurge(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()
	before := time.Now().UTC()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionDelete, activeWorkspace(workspaceID))
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(activeWorkspace(workspaceID), nil)

	var requestedAt, purgeAfter time.Time

	h.workspaces.EXPECT().
		MarkPendingDeletion(gomock.Any(), workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, requested, purge time.Time) (entity.Workspace, error) {
			requestedAt = requested
			purgeAfter = purge

			pending := activeWorkspace(id)
			pending.Status = entity.WorkspaceStatusPendingDeletion
			pending.DeletionRequestedAt = &requested
			pending.PurgeAfter = &purge

			return pending, nil
		})

	var scheduledFor time.Time

	h.producer.EXPECT().
		EnqueueWorkspacePurge(gomock.Any(), entity.WorkspacePurgePayload{WorkspaceID: workspaceID}, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.WorkspacePurgePayload, processAt time.Time) error {
			scheduledFor = processAt

			return nil
		})

	deleted, err := h.service.Delete(actingAs(actorID), workspaceID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if !deleted.Deleted() {
		t.Fatal("the workspace should be pending deletion, not gone")
	}

	if got := purgeAfter.Sub(requestedAt); got != deletionGracePeriod {
		t.Fatalf("recovery window = %v, want the configured %v", got, deletionGracePeriod)
	}

	if requestedAt.Before(before) {
		t.Error("deletion was recorded before the request was made")
	}

	if !scheduledFor.Equal(purgeAfter) {
		t.Fatalf("purge scheduled for %v, want the purge date %v", scheduledFor, purgeAfter)
	}
}

func TestDeletingTwiceIsRefusedSoTheWindowCannotBeExtended(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	pending := pendingWorkspace(workspaceID, time.Now().UTC().Add(-time.Hour))

	h.expectActorMayAct(workspaceID, actorID, entity.ActionDelete, pending)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(pending, nil)

	_, err := h.service.Delete(actingAs(actorID), workspaceID)
	if !errors.Is(err, entity.ErrWorkspaceDeleted) {
		t.Fatalf("Delete error = %v, want ErrWorkspaceDeleted", err)
	}

	var deleted entity.WorkspaceDeletedError
	if !errors.As(err, &deleted) || deleted.PurgeAfter == nil {
		t.Fatal("the refusal should carry the existing purge date so the caller can show it")
	}
}

func TestADeletedWorkspaceRefusesOrdinaryWorkButStillRestores(t *testing.T) {
	operations := []struct {
		name string
		call func(h *harness, ctx context.Context, workspaceID uuid.UUID) error
	}{
		{
			name: "adding a member",
			call: func(h *harness, ctx context.Context, workspaceID uuid.UUID) error {
				_, err := h.service.AddMember(ctx, workspaceID, uuid.New(), entity.MembershipRoleMember)

				return err
			},
		},
		{
			name: "changing a member role",
			call: func(h *harness, ctx context.Context, workspaceID uuid.UUID) error {
				_, err := h.service.ChangeMemberRole(ctx, workspaceID, uuid.New(), entity.MembershipRoleAdmin)

				return err
			},
		},
		{
			name: "removing a member",
			call: func(h *harness, ctx context.Context, workspaceID uuid.UUID) error {
				return h.service.RemoveMember(ctx, workspaceID, uuid.New(), nil)
			},
		},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			h := newHarness(t)
			workspaceID := uuid.New()
			actorID := uuid.New()

			h.expectActorMayManageMembersOn(workspaceID, actorID, entity.WorkspaceStatusPendingDeletion)

			err := operation.call(h, actingAs(actorID), workspaceID)
			if !errors.Is(err, entity.ErrWorkspaceDeleted) {
				t.Fatalf("%s error = %v, want ErrWorkspaceDeleted", operation.name, err)
			}
		})
	}
}

func TestADeletedWorkspaceRefusesAnAuthPolicyChange(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	h.expectActorMayActOn(
		workspaceID,
		actorID,
		entity.ResourceAuthPolicy,
		entity.ActionUpdate,
		workspaceWithStatus(workspaceID, entity.WorkspaceStatusPendingDeletion),
	)

	_, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcementSSO)
	if !errors.Is(err, entity.ErrWorkspaceDeleted) {
		t.Fatalf("SetAuthPolicy error = %v, want ErrWorkspaceDeleted", err)
	}
}

func TestRestoreBringsADeletedWorkspaceBack(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionDelete, pendingWorkspace(workspaceID, time.Now().UTC()))
	h.workspaces.EXPECT().Restore(gomock.Any(), workspaceID).Return(activeWorkspace(workspaceID), nil)

	restored, err := h.service.Restore(actingAs(actorID), workspaceID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if restored.Deleted() {
		t.Fatal("the restored workspace is still pending deletion")
	}
}

func TestRestoringALiveWorkspaceIsRefused(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionDelete, activeWorkspace(workspaceID))
	h.workspaces.EXPECT().
		Restore(gomock.Any(), workspaceID).
		Return(entity.Workspace{}, entity.ErrWorkspaceNotDeleted)

	_, err := h.service.Restore(actingAs(actorID), workspaceID)
	if !errors.Is(err, entity.ErrWorkspaceNotDeleted) {
		t.Fatalf("Restore error = %v, want ErrWorkspaceNotDeleted", err)
	}
}

func TestPurgeDestroysTheWorkspaceOnlyOnceItsWindowHasElapsed(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()

	pending := pendingWorkspace(workspaceID, time.Now().UTC().Add(-2*deletionGracePeriod))

	emptied := false

	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(pending, nil)
	h.blobs.EXPECT().
		RemoveAll(gomock.Any(), entity.AttachmentPrefix(workspaceID)).
		DoAndReturn(func(context.Context, string) error {
			emptied = true

			return nil
		})
	h.workspaces.EXPECT().
		Purge(gomock.Any(), workspaceID).
		DoAndReturn(func(context.Context, uuid.UUID) error {
			if !emptied {
				t.Fatal(
					"the workspace rows went before its stored files. A cascade cannot reach " +
						"object storage, so the only record of which bytes to delete would be " +
						"gone and they would be paid for forever.",
				)
			}

			return nil
		})

	if err := h.service.Purge(context.Background(), workspaceID); err != nil {
		t.Fatalf("Purge: %v", err)
	}
}

func TestPurgeIsAbandonedWhenTheStoredFilesCannotBeRemoved(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()

	pending := pendingWorkspace(workspaceID, time.Now().UTC().Add(-2*deletionGracePeriod))
	unreachable := errors.New("storage is unreachable")

	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(pending, nil)
	h.blobs.EXPECT().RemoveAll(gomock.Any(), gomock.Any()).Return(unreachable)

	if err := h.service.Purge(context.Background(), workspaceID); !errors.Is(err, unreachable) {
		t.Fatalf(
			"Purge returned %v. When storage is down the transaction has to fail so asynq retries; "+
				"committing anyway would strand the bytes with nothing left that names them.",
			err,
		)
	}
}

func TestPurgeSparesAWorkspaceThatWasRestored(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()

	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(activeWorkspace(workspaceID), nil)

	if err := h.service.Purge(context.Background(), workspaceID); err != nil {
		t.Fatalf("Purge: %v", err)
	}
}

func TestPurgeSparesAWorkspaceStillInsideItsWindow(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()

	pending := pendingWorkspace(workspaceID, time.Now().UTC())

	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(pending, nil)

	if err := h.service.Purge(context.Background(), workspaceID); err != nil {
		t.Fatalf("Purge: %v", err)
	}
}

func TestPurgeIgnoresAWorkspaceThatIsAlreadyGone(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()

	h.workspaces.EXPECT().
		GetByID(gomock.Any(), workspaceID).
		Return(entity.Workspace{}, entity.ErrWorkspaceNotFound)

	if err := h.service.Purge(context.Background(), workspaceID); err != nil {
		t.Fatalf("a redelivered purge for a vanished workspace must not retry forever, got %v", err)
	}
}

func TestCreateSeedsTheFirstTeamAndMakesItTheDefault(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	teamID := uuid.New()

	h.workspaces.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspace entity.Workspace) (entity.Workspace, error) {
			workspace.ID = workspaceID

			return workspace, nil
		})
	h.memberships.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Membership{}, nil)

	var capturedTeam entity.Team

	h.teams.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, team entity.Team) (entity.Team, error) {
			capturedTeam = team
			team.ID = teamID

			return team, nil
		})

	var seededStates []entity.WorkflowState

	h.states.EXPECT().
		CreateMany(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, states []entity.WorkflowState) ([]entity.WorkflowState, error) {
			seededStates = states

			return states, nil
		})

	var capturedMembership entity.TeamMembership

	h.teamMembers.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, membership entity.TeamMembership) (entity.TeamMembership, error) {
			capturedMembership = membership

			return membership, nil
		})

	var capturedDefault *uuid.UUID

	h.workspaces.EXPECT().
		UpdateSettings(gomock.Any(), workspaceID, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, name, timezone string, defaultTeamID *uuid.UUID) (entity.Workspace, error) {
			capturedDefault = defaultTeamID

			seeded := activeWorkspace(id)
			seeded.DefaultTeamID = defaultTeamID

			return seeded, nil
		})

	workspace, err := h.service.Create(actingAs(actorID), service.CreateWorkspaceInput{
		Slug: "northwind",
		Name: "Northwind",
		Team: &service.CreateWorkspaceTeamInput{Key: "MOB", Name: "Mobile"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if capturedTeam.Key != "MOB" || capturedTeam.WorkspaceID != workspaceID {
		t.Fatalf("seeded team = %+v, want MOB in the new workspace", capturedTeam)
	}

	if capturedMembership.AccountID != actorID || capturedMembership.TeamID != teamID {
		t.Fatalf("team membership = %+v, want the creator on the first team", capturedMembership)
	}

	assertUsableStateSet(t, seededStates)

	if capturedDefault == nil || *capturedDefault != teamID {
		t.Fatalf("default team = %v, want the first team", capturedDefault)
	}

	if workspace.DefaultTeamID == nil || *workspace.DefaultTeamID != teamID {
		t.Fatalf("returned default team = %v, want %v", workspace.DefaultTeamID, teamID)
	}
}

func TestCreateWithoutATeamLeavesTheWorkspaceWithoutADefault(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()

	h.workspaces.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspace entity.Workspace) (entity.Workspace, error) {
			workspace.ID = uuid.New()

			return workspace, nil
		})
	h.memberships.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Membership{}, nil)

	workspace, err := h.service.Create(actingAs(actorID), service.CreateWorkspaceInput{
		Slug: "northwind",
		Name: "Northwind",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if workspace.DefaultTeamID != nil {
		t.Fatalf("default team = %v, want none when no team was requested", workspace.DefaultTeamID)
	}
}

func TestUpdateRejectsADefaultTeamFromAnotherWorkspace(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	actorID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))
	h.teams.EXPECT().GetByID(gomock.Any(), teamID).Return(entity.Team{
		ID:          teamID,
		WorkspaceID: uuid.New(),
		Status:      entity.TeamStatusActive,
	}, nil)

	_, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{DefaultTeamID: &teamID})

	assertDefaultTeamRejected(t, err)
}

func TestUpdateRejectsAnArchivedDefaultTeam(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	actorID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))
	h.teams.EXPECT().GetByID(gomock.Any(), teamID).Return(entity.Team{
		ID:          teamID,
		WorkspaceID: workspaceID,
		Status:      entity.TeamStatusArchived,
	}, nil)

	_, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{DefaultTeamID: &teamID})

	assertDefaultTeamRejected(t, err)
}

func TestUpdateRejectsADefaultTeamThatDoesNotExist(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	actorID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))
	h.teams.EXPECT().GetByID(gomock.Any(), teamID).Return(entity.Team{}, entity.ErrTeamNotFound)

	_, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{DefaultTeamID: &teamID})

	assertDefaultTeamRejected(t, err)
}

func TestUpdateAssignsADefaultTeamThatBelongsToTheWorkspace(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	actorID := uuid.New()
	teamID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))
	h.teams.EXPECT().GetByID(gomock.Any(), teamID).Return(entity.Team{
		ID:          teamID,
		WorkspaceID: workspaceID,
		Status:      entity.TeamStatusActive,
		Visibility:  entity.TeamVisibilityPrivate,
	}, nil)

	var capturedDefault *uuid.UUID

	h.workspaces.EXPECT().
		UpdateSettings(gomock.Any(), workspaceID, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, name, timezone string, defaultTeamID *uuid.UUID) (entity.Workspace, error) {
			capturedDefault = defaultTeamID

			return activeWorkspace(id), nil
		})

	if _, err := h.service.Update(
		actingAs(actorID),
		workspaceID,
		service.UpdateWorkspaceInput{DefaultTeamID: &teamID},
	); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if capturedDefault == nil || *capturedDefault != teamID {
		t.Fatalf("wrote default team = %v, want %v; a private team is a legitimate default", capturedDefault, teamID)
	}
}

func assertDefaultTeamRejected(t *testing.T, err error) {
	t.Helper()

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Update error = %v, want a ValidationError", err)
	}

	if validation.Fields[0].Field != "defaultTeamId" {
		t.Fatalf("field = %q, want the error attributed to defaultTeamId", validation.Fields[0].Field)
	}
}

func TestNoLayerCountsWorkspacesOrTheirMembers(t *testing.T) {
	forbidden := []string{"count", "total", "quota", "seat", "limit"}

	surfaces := map[string]reflect.Type{
		"repository.Workspace":  reflect.TypeOf((*repository.Workspace)(nil)).Elem(),
		"repository.Membership": reflect.TypeOf((*repository.Membership)(nil)).Elem(),
		"service.Workspaces":    reflect.TypeOf((*service.Workspaces)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := strings.ReplaceAll(strings.ToLower(surface.Method(i).Name), "account", "")

			for _, word := range forbidden {
				if strings.Contains(method, word) {
					t.Errorf("%s exposes %q, which counts or caps workspace contents", name, surface.Method(i).Name)
				}
			}
		}
	}

	page := reflect.TypeOf(service.MemberPage{})
	for i := range page.NumField() {
		field := strings.ToLower(page.Field(i).Name)

		for _, word := range forbidden {
			if strings.Contains(field, word) {
				t.Errorf("service.MemberPage carries %q, which would put a member count on the wire", page.Field(i).Name)
			}
		}
	}
}

func assertUsableStateSet(t *testing.T, states []entity.WorkflowState) {
	t.Helper()

	if len(states) == 0 {
		t.Fatal("a new team was seeded with no workflow states, so it is not usable without configuration")
	}

	present := make(map[entity.StateCategory]bool, len(states))

	var defaults, completions int

	for _, state := range states {
		present[state.Category] = true

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

	if defaults != 1 {
		t.Errorf("the seeded set has %d default states, want exactly 1", defaults)
	}

	if completions != 1 {
		t.Errorf("the seeded set has %d completion states, want exactly 1", completions)
	}
}

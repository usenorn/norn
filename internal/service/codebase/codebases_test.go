package codebase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

func TestOnlyARunnerMayConnectACodebase(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.Connect(h.asPerson(), h.connecting(repositoryAt("api", "api")))

	if !errors.Is(err, entity.ErrCodebaseNotRunner) {
		t.Fatalf("a signed-in person connecting a codebase returned %v, want %v",
			err, entity.ErrCodebaseNotRunner)
	}
}

func TestARevokedRunnerCannotConnectACodebase(t *testing.T) {
	h := newHarness(t)
	h.runner.Status = entity.RunnerStatusRevoked

	_, err := h.service.Connect(h.asRunner(), h.connecting(repositoryAt("api", "api")))

	if !errors.Is(err, entity.ErrRunnerRevoked) {
		t.Fatalf("a revoked runner connecting a codebase returned %v, want %v",
			err, entity.ErrRunnerRevoked)
	}
}

func TestAFolderTheRunnerHasNotSeenBeforeIsConnectedFresh(t *testing.T) {
	h := newHarness(t)
	api := repositoryAt("api", "api")

	h.codebases.EXPECT().
		GetLiveByRoot(gomock.Any(), h.runner.ID, "/Users/vlad/projects/norn").
		Return(entity.Codebase{}, entity.ErrCodebaseNotFound)

	var captured repository.CodebaseInventory

	h.codebases.EXPECT().
		Connect(gomock.Any(), h.runner.ID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, inventory repository.CodebaseInventory, _ time.Time,
		) (entity.Codebase, error) {
			captured = inventory

			return h.live(entity.CodebaseStateActive, inventory.Repositories...), nil
		})

	codebase, err := h.service.Connect(h.asRunner(), h.connecting(api))
	if err != nil {
		t.Fatalf("connect a new codebase: %v", err)
	}

	if codebase.State != entity.CodebaseStateActive {
		t.Fatalf("a freshly connected codebase is %q, want %q", codebase.State, entity.CodebaseStateActive)
	}

	if len(captured.Repositories) != 1 || captured.Repositories[0].RelPath != "api" {
		t.Fatalf("the inventory handed to the repository was %+v, want the one repository reported", captured.Repositories)
	}

	if captured.Repositories[0].Remote.Hash != "api-hash" {
		t.Fatal("the remote fingerprint did not survive into the stored inventory")
	}
}

func TestReportingTheSameRepositoriesLeavesTheCodebaseAlone(t *testing.T) {
	h := newHarness(t)
	api := repositoryAt("api", "api")
	web := repositoryAt("web", "web")

	h.codebases.EXPECT().
		GetLiveByRoot(gomock.Any(), h.runner.ID, gomock.Any()).
		Return(h.live(entity.CodebaseStateActive, web, api), nil)

	h.codebases.EXPECT().
		Replace(gomock.Any(), gomock.Any(), gomock.Any(), entity.CodebaseStateActive, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, inventory repository.CodebaseInventory,
			state entity.CodebaseState, _ time.Time,
		) (entity.Codebase, error) {
			return h.live(state, inventory.Repositories...), nil
		})

	codebase, err := h.service.Connect(h.asRunner(), h.connecting(api, web))
	if err != nil {
		t.Fatalf("re-report an unchanged codebase: %v", err)
	}

	if codebase.State != entity.CodebaseStateActive {
		t.Fatalf("re-reporting the same repositories in a different order gave %q, want %q",
			codebase.State, entity.CodebaseStateActive)
	}
}

func TestReportingADifferentRepositorySetRaisesDrift(t *testing.T) {
	h := newHarness(t)
	api := repositoryAt("api", "api")
	web := repositoryAt("web", "web")

	h.codebases.EXPECT().
		GetLiveByRoot(gomock.Any(), h.runner.ID, gomock.Any()).
		Return(h.live(entity.CodebaseStateActive, api), nil)

	h.codebases.EXPECT().
		Replace(gomock.Any(), gomock.Any(), gomock.Any(), entity.CodebaseStateDrift, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, inventory repository.CodebaseInventory,
			state entity.CodebaseState, _ time.Time,
		) (entity.Codebase, error) {
			return h.live(state, inventory.Repositories...), nil
		})

	codebase, err := h.service.Connect(h.asRunner(), h.connecting(api, web))
	if err != nil {
		t.Fatalf("re-report a changed codebase: %v", err)
	}

	if codebase.State != entity.CodebaseStateDrift {
		t.Fatalf("adding a repository gave %q, want %q", codebase.State, entity.CodebaseStateDrift)
	}
}

func TestARunnerCannotTouchACodebaseHeldByAnotherMachine(t *testing.T) {
	h := newHarness(t)

	elsewhere := h.live(entity.CodebaseStateDrift)
	elsewhere.RunnerID = uuid.New()

	h.codebases.EXPECT().GetByID(gomock.Any(), elsewhere.ID).Return(elsewhere, nil).AnyTimes()

	ctx := h.asRunner()

	if _, err := h.service.Confirm(ctx, elsewhere.ID); !errors.Is(err, entity.ErrCodebaseNotFound) {
		t.Fatalf("confirming another machine's codebase returned %v, want %v",
			err, entity.ErrCodebaseNotFound)
	}

	if _, err := h.service.Disconnect(ctx, elsewhere.ID); !errors.Is(err, entity.ErrCodebaseNotFound) {
		t.Fatalf("disconnecting another machine's codebase returned %v, want %v",
			err, entity.ErrCodebaseNotFound)
	}
}

func TestADisconnectedCodebaseCannotBeConfirmed(t *testing.T) {
	h := newHarness(t)

	gone := h.live(entity.CodebaseStateDisconnected)
	h.codebases.EXPECT().GetByID(gomock.Any(), gone.ID).Return(gone, nil)

	_, err := h.service.Confirm(h.asRunner(), gone.ID)

	if !errors.Is(err, entity.ErrCodebaseDisconnected) {
		t.Fatalf("confirming a disconnected codebase returned %v, want %v",
			err, entity.ErrCodebaseDisconnected)
	}
}

func TestConfirmingClearsDriftThroughTheRepository(t *testing.T) {
	h := newHarness(t)

	drifted := h.live(entity.CodebaseStateDrift)
	confirmed := drifted
	confirmed.State = entity.CodebaseStateActive

	h.codebases.EXPECT().GetByID(gomock.Any(), drifted.ID).Return(drifted, nil)
	h.codebases.EXPECT().Confirm(gomock.Any(), drifted.ID, gomock.Any()).Return(confirmed, nil)

	codebase, err := h.service.Confirm(h.asRunner(), drifted.ID)
	if err != nil {
		t.Fatalf("confirm a drifted codebase: %v", err)
	}

	if codebase.State != entity.CodebaseStateActive {
		t.Fatalf("confirming left the codebase %q, want %q", codebase.State, entity.CodebaseStateActive)
	}
}

func TestTheOwnerOfAnAgentSeesItsCodebasesAndAStrangerDoesNot(t *testing.T) {
	h := newHarness(t)
	held := []entity.Codebase{h.live(entity.CodebaseStateActive)}

	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, h.agent.ID).Return(h.agent, nil).AnyTimes()
	h.codebases.EXPECT().ListByAgentID(gomock.Any(), h.agent.ID).Return(held, nil).AnyTimes()

	listed, err := h.service.ListByAgent(h.asPerson(), h.workspaceID, h.agent.ID)
	if err != nil {
		t.Fatalf("the agent's owner listing its codebases: %v", err)
	}

	if len(listed) != 1 {
		t.Fatalf("the owner saw %d codebases, want 1", len(listed))
	}

	h.caller = uuid.New()

	if _, err := h.service.ListByAgent(h.asPerson(), h.workspaceID, h.agent.ID); !errors.Is(err, entity.ErrAgentNotFound) {
		t.Fatalf("a member who does not own the agent got %v, want %v — anything else tells them it exists",
			err, entity.ErrAgentNotFound)
	}
}

func TestAnAdministratorSeesTheCodebasesOfAnAgentTheyDoNotOwn(t *testing.T) {
	h := newHarness(t)
	h.caller = uuid.New()
	h.role = entity.MembershipRoleAdmin

	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, h.agent.ID).Return(h.agent, nil)
	h.codebases.EXPECT().
		ListByAgentID(gomock.Any(), h.agent.ID).
		Return([]entity.Codebase{h.live(entity.CodebaseStateActive)}, nil)

	listed, err := h.service.ListByAgent(h.asPerson(), h.workspaceID, h.agent.ID)
	if err != nil {
		t.Fatalf("an administrator listing an agent's codebases: %v", err)
	}

	if len(listed) != 1 {
		t.Fatalf("the administrator saw %d codebases, want 1", len(listed))
	}
}

func TestDisconnectingIsSoftSoTheRecordSurvives(t *testing.T) {
	h := newHarness(t)

	live := h.live(entity.CodebaseStateActive)
	gone := live
	gone.State = entity.CodebaseStateDisconnected

	h.codebases.EXPECT().GetByID(gomock.Any(), live.ID).Return(live, nil)
	h.codebases.EXPECT().Disconnect(gomock.Any(), live.ID, gomock.Any()).Return(gone, nil)

	codebase, err := h.service.Disconnect(h.asRunner(), live.ID)
	if err != nil {
		t.Fatalf("disconnect a codebase: %v", err)
	}

	if codebase.State != entity.CodebaseStateDisconnected || codebase.ID != live.ID {
		t.Fatalf("disconnect returned %+v, want the same codebase marked disconnected", codebase)
	}
}

func TestAnInventoryTheRunnerCannotDescribeIsRefusedBeforeItIsStored(t *testing.T) {
	h := newHarness(t)

	input := h.connecting(entity.CodebaseRepository{Name: "", RelPath: "api"})

	_, err := h.service.Connect(h.asRunner(), input)

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("a repository with no name returned %v, want a validation error", err)
	}
}

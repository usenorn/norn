package project_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) expectProjectUpdate(captured *repository.ProjectSettings) {
	h.projects.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, h.projectID).
		Return(h.project(entity.ProjectStateActive), nil)
	h.projects.EXPECT().
		UpdateSettings(gomock.Any(), h.projectID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, settings repository.ProjectSettings) (entity.Project, error) {
			*captured = settings

			updated := h.project(entity.ProjectStateActive)
			if settings.AgentInstructions != nil {
				updated.AgentInstructions = *settings.AgentInstructions
			}

			return updated, nil
		})
	h.projects.EXPECT().HasConcealedWork(gomock.Any(), gomock.Any(), h.projectID).Return(false, nil)
}

func TestProjectAgentInstructionsAreTrimmedBeforeTheyAreStored(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleAdmin, uuid.New())

	var captured repository.ProjectSettings

	h.expectProjectUpdate(&captured)

	padded := "\n  Touch the ledger only.  \n"

	view, err := h.service.Update(context.Background(), h.workspaceID, h.projectID, service.UpdateProjectInput{
		AgentInstructions: &padded,
	})
	if err != nil {
		t.Fatalf("Update error = %v", err)
	}

	if captured.AgentInstructions == nil || *captured.AgentInstructions != "Touch the ledger only." {
		t.Fatalf(
			"the repository was handed %v. Trimming in the browser leaves a direct API client "+
				"storing the padding.",
			captured.AgentInstructions,
		)
	}

	if view.Project.AgentInstructions != "Touch the ledger only." {
		t.Errorf("read back %q, want the trimmed body", view.Project.AgentInstructions)
	}
}

func TestAnEditThatSaysNothingAboutInstructionsLeavesThemAlone(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleAdmin, uuid.New())

	var captured repository.ProjectSettings

	h.expectProjectUpdate(&captured)

	name := "Checkout rebuild II"

	if _, err := h.service.Update(context.Background(), h.workspaceID, h.projectID, service.UpdateProjectInput{
		Name: &name,
	}); err != nil {
		t.Fatalf("Update error = %v", err)
	}

	if captured.AgentInstructions != nil {
		t.Fatalf(
			"renaming the project carried instructions of %q down to the store. An omitted "+
				"field has to reach the query as nothing at all, or the update blanks it.",
			*captured.AgentInstructions,
		)
	}
}

func TestWritingOnlyInstructionsSaysNothingAboutTheDescription(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleAdmin, uuid.New())

	var captured repository.ProjectSettings

	h.expectProjectUpdate(&captured)

	written := "Touch the ledger only."

	if _, err := h.service.Update(context.Background(), h.workspaceID, h.projectID, service.UpdateProjectInput{
		AgentInstructions: &written,
	}); err != nil {
		t.Fatalf("Update error = %v", err)
	}

	if captured.Description != nil {
		t.Fatalf(
			"writing instructions carried a description of %q to the store. An edit that says "+
				"nothing about the description must leave whatever is there alone.",
			*captured.Description,
		)
	}
}

func TestADescriptionAskedForInSoManyWordsStillReachesTheStore(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleAdmin, uuid.New())

	var captured repository.ProjectSettings

	h.expectProjectUpdate(&captured)

	emptied := ""

	if _, err := h.service.Update(context.Background(), h.workspaceID, h.projectID, service.UpdateProjectInput{
		Description: &emptied,
	}); err != nil {
		t.Fatalf("Update error = %v", err)
	}

	if captured.Description == nil || *captured.Description != "" {
		t.Fatalf("the store was handed %v, want an empty description", captured.Description)
	}
}

func TestProjectInstructionsPastTheLimitNeverReachTheStore(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.MembershipRoleAdmin, uuid.New())

	h.projects.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, h.projectID).
		Return(h.project(entity.ProjectStateActive), nil)
	h.projects.EXPECT().UpdateSettings(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	tooMany := strings.Repeat("a", entity.AgentInstructionsMaxLen+1)

	_, err := h.service.Update(context.Background(), h.workspaceID, h.projectID, service.UpdateProjectInput{
		AgentInstructions: &tooMany,
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Update error = %v, want a validation error", err)
	}
}

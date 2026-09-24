package agent_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func ownedAgent(h *harness) entity.Agent {
	return entity.Agent{
		ID:             uuid.New(),
		WorkspaceID:    h.workspaceID,
		AccountID:      uuid.New(),
		OwnerAccountID: h.adminID,
		Name:           "triage-bot",
	}
}

func expectDescribable(h *harness, agent entity.Agent) {
	h.accounts.EXPECT().
		GetByID(gomock.Any(), agent.OwnerAccountID).
		Return(entity.Account{ID: agent.OwnerAccountID, DisplayName: "Rae"}, nil)
	h.tokens.EXPECT().
		GetLatestByOwner(gomock.Any(), agent.AccountID).
		Return(entity.APIToken{
			Scopes: readScopes(),
			Grants: entity.APITokenGrants{{WorkspaceID: h.workspaceID, AllTeams: true}},
		}, nil)
}

func TestAgentInstructionsAreTrimmedBeforeTheyAreStored(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	agent := ownedAgent(h)

	var stored string

	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, agent.ID).Return(agent, nil)
	h.agents.EXPECT().
		SetInstructions(gomock.Any(), h.workspaceID, agent.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, instructions string) (entity.Agent, error) {
			stored = instructions
			agent.AgentInstructions = instructions

			return agent, nil
		})
	expectDescribable(h, agent)

	saved, err := h.service.SetInstructions(context.Background(), service.SetAgentInstructionsInput{
		WorkspaceID:  h.workspaceID,
		AgentID:      agent.ID,
		Instructions: "\n  Ask before deleting.  \n",
	})
	if err != nil {
		t.Fatalf("SetInstructions: %v", err)
	}

	if stored != "Ask before deleting." {
		t.Fatalf(
			"the repository was handed %q. The browser is not the only caller, so a direct API "+
				"client must see the same trimming it does.",
			stored,
		)
	}

	if saved.Agent.AgentInstructions != "Ask before deleting." {
		t.Errorf("read back %q, want the trimmed body", saved.Agent.AgentInstructions)
	}
}

func TestInstructionsOfNothingButWhitespaceClearTheAgentsOwn(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	agent := ownedAgent(h)
	agent.AgentInstructions = "Ask before deleting."

	var stored string

	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, agent.ID).Return(agent, nil)
	h.agents.EXPECT().
		SetInstructions(gomock.Any(), h.workspaceID, agent.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, instructions string) (entity.Agent, error) {
			stored = instructions
			agent.AgentInstructions = instructions

			return agent, nil
		})
	expectDescribable(h, agent)

	if _, err := h.service.SetInstructions(context.Background(), service.SetAgentInstructionsInput{
		WorkspaceID:  h.workspaceID,
		AgentID:      agent.ID,
		Instructions: "   \n\t ",
	}); err != nil {
		t.Fatalf("SetInstructions: %v", err)
	}

	if stored != "" {
		t.Fatalf("the repository was handed %q, want the setting cleared", stored)
	}
}

func TestInstructionsPastTheLimitNeverReachTheStore(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	h.agents.EXPECT().SetInstructions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, err := h.service.SetInstructions(context.Background(), service.SetAgentInstructionsInput{
		WorkspaceID:  h.workspaceID,
		AgentID:      uuid.New(),
		Instructions: strings.Repeat("a", entity.AgentInstructionsMaxLen+1),
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("SetInstructions error = %v, want a validation error", err)
	}
}

func TestAnAgentSomebodyElseActsForHasNoInstructionsToWrite(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleMember)

	theirs := entity.Agent{
		ID:             uuid.New(),
		WorkspaceID:    h.workspaceID,
		AccountID:      uuid.New(),
		OwnerAccountID: uuid.New(),
		Name:           "theirs",
	}

	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, theirs.ID).Return(theirs, nil)
	h.agents.EXPECT().SetInstructions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	if _, err := h.service.SetInstructions(context.Background(), service.SetAgentInstructionsInput{
		WorkspaceID:  h.workspaceID,
		AgentID:      theirs.ID,
		Instructions: "Ask before deleting.",
	}); !errors.Is(err, entity.ErrAgentNotFound) {
		t.Fatalf("SetInstructions error = %v, want ErrAgentNotFound", err)
	}
}

func TestWritingAgentInstructionsIsRecordedInTheAudit(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	agent := ownedAgent(h)

	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, agent.ID).Return(agent, nil)
	h.agents.EXPECT().
		SetInstructions(gomock.Any(), h.workspaceID, agent.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, instructions string) (entity.Agent, error) {
			agent.AgentInstructions = instructions

			return agent, nil
		})
	expectDescribable(h, agent)

	if _, err := h.service.SetInstructions(context.Background(), service.SetAgentInstructionsInput{
		WorkspaceID:  h.workspaceID,
		AgentID:      agent.ID,
		Instructions: "Ask before deleting.",
	}); err != nil {
		t.Fatalf("SetInstructions: %v", err)
	}

	if len(h.recorded) != 1 ||
		h.recorded[0].Action != entity.AuditAgentInstructions ||
		h.recorded[0].ResourceID != agent.ID {
		t.Fatalf(
			"the audit recorded %+v. Changing what an agent is told is a change to what it may "+
				"do, so it has to be as legible as disabling one.",
			h.recorded,
		)
	}
}

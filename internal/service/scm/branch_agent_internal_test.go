package scm

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
)

type branchHarness struct {
	settings *scmrepo.MockSCMTeamSetting
	agents   *agentrepo.MockAgent
	accounts *accountrepo.MockAccount
	service  *connections

	issue entity.Issue
	agent entity.Agent
}

func newBranchHarness(t *testing.T, template string) *branchHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspaceID := uuid.New()
	ownerID := uuid.New()

	h := &branchHarness{
		settings: scmrepo.NewMockSCMTeamSetting(ctrl),
		agents:   agentrepo.NewMockAgent(ctrl),
		accounts: accountrepo.NewMockAccount(ctrl),
		issue: entity.Issue{
			ID:           uuid.New(),
			WorkspaceID:  workspaceID,
			TeamID:       uuid.New(),
			ReferenceKey: "NORN",
			Number:       74,
			Title:        "Gate the pull request body",
		},
		agent: entity.Agent{
			ID:             uuid.New(),
			WorkspaceID:    workspaceID,
			OwnerAccountID: ownerID,
			Name:           "opsy",
		},
	}

	h.service = &connections{
		teamSettings: h.settings,
		agents:       h.agents,
		accounts:     h.accounts,
	}

	h.settings.EXPECT().
		Get(gomock.Any(), workspaceID, h.issue.TeamID).
		Return(entity.SCMTeamSettings{BranchTemplate: template}, nil).
		AnyTimes()

	return h
}

func (h *branchHarness) owned(displayName string) {
	h.agents.EXPECT().
		GetByID(gomock.Any(), h.issue.WorkspaceID, h.agent.ID).
		Return(h.agent, nil).
		AnyTimes()

	h.accounts.EXPECT().
		GetByID(gomock.Any(), h.agent.OwnerAccountID).
		Return(entity.Account{ID: h.agent.OwnerAccountID, DisplayName: displayName}, nil).
		AnyTimes()
}

func (h *branchHarness) name(t *testing.T) string {
	t.Helper()

	branch, err := h.service.BranchNameForAgent(context.Background(), h.issue, h.agent.ID)
	if err != nil {
		t.Fatalf("naming the branch for a run: %v", err)
	}

	return branch
}

func TestARunBranchesUnderThePersonWhoOwnsTheAgent(t *testing.T) {
	h := newBranchHarness(t, "")
	h.owned("Rae Chen")

	if got, want := h.name(t), "rae-chen/norn-74-gate-the-pull-request-body"; got != want {
		t.Fatalf(
			"a run branched as %q, want %q. It has to match what norn hands a person for the "+
				"same issue, or the two halves of the product disagree about what the branch is "+
				"called",
			got, want,
		)
	}
}

func TestARunHonoursTheTemplateTheTeamChose(t *testing.T) {
	h := newBranchHarness(t, "feature/{reference}")
	h.owned("Rae Chen")

	if got, want := h.name(t), "feature/norn-74"; got != want {
		t.Fatalf(
			"a run branched as %q, want %q. A team that set its own template did so for every "+
				"branch, not only the ones a person types",
			got, want,
		)
	}
}

func TestAnAgentNobodyCanResolveStillGetsAUsableBranch(t *testing.T) {
	h := newBranchHarness(t, "")

	h.agents.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Agent{}, errors.New("no such agent")).
		AnyTimes()

	if got, want := h.name(t), "norn/norn-74-gate-the-pull-request-body"; got != want {
		t.Fatalf(
			"an agent that could not be read branched as %q, want %q. Naming a branch must not "+
				"be the thing that stops a run",
			got, want,
		)
	}
}

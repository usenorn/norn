package agenthold

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service/checkgate"
)

type Gate struct {
	settings  repository.AgentSetting
	proposals repository.AgentProposal
	agents    repository.Agent
	states    repository.WorkflowState
	checks    *checkgate.Gate
}

func New(
	settings repository.AgentSetting,
	proposals repository.AgentProposal,
	agents repository.Agent,
	states repository.WorkflowState,
	checks *checkgate.Gate,
) *Gate {
	return &Gate{
		settings:  settings,
		proposals: proposals,
		agents:    agents,
		states:    states,
		checks:    checks,
	}
}

func (g *Gate) Hold(
	ctx context.Context,
	decision entity.Decision,
	issue entity.Issue,
	actions []entity.AgentAction,
	change entity.AgentChange,
) (entity.AgentProposal, bool, error) {
	if decision.Actor.Kind != entity.ActorKindAgent || identity.Approved(ctx) {
		return entity.AgentProposal{}, false, nil
	}

	if len(actions) == 0 {
		return entity.AgentProposal{}, false, nil
	}

	configured, err := g.settings.Settings(ctx, issue.WorkspaceID, issue.TeamID)
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	action, policy := configured.Strongest(actions)

	switch policy {
	case entity.AgentHoldNever:
		return entity.AgentProposal{}, false, nil

	case entity.AgentHoldUnlessProven:
		waits, err := g.unproven(ctx, issue, change)
		if err != nil {
			return entity.AgentProposal{}, false, err
		}

		if !waits {
			return entity.AgentProposal{}, false, nil
		}
	}

	agent, err := g.agents.GetByAccountID(ctx, decision.Actor.AccountID)
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	change.ExpectedVersion = issue.Version

	held, err := g.proposals.Create(ctx, entity.AgentProposal{
		WorkspaceID: issue.WorkspaceID,
		AgentID:     agent.ID,
		IssueID:     issue.ID,
		TeamID:      issue.TeamID,
		Action:      action,
		Change:      change,
	})
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	return held, true, nil
}

func (g *Gate) unproven(
	ctx context.Context,
	issue entity.Issue,
	change entity.AgentChange,
) (bool, error) {
	completing, err := g.completing(ctx, issue, change)
	if err != nil || !completing {
		return false, err
	}

	blocking, err := g.checks.Blocking(ctx, issue.WorkspaceID, issue.ID)
	if err != nil {
		return false, err
	}

	return len(blocking) > 0, nil
}

func (g *Gate) completing(
	ctx context.Context,
	issue entity.Issue,
	change entity.AgentChange,
) (bool, error) {
	if change.StateID == nil || *change.StateID == uuid.Nil {
		return false, nil
	}

	if issue.State.Category == entity.StateCategoryComplete {
		return false, nil
	}

	states, err := g.states.ListByTeamID(ctx, issue.TeamID)
	if err != nil {
		return false, err
	}

	for _, state := range states {
		if state.ID == *change.StateID {
			return state.Category == entity.StateCategoryComplete, nil
		}
	}

	return false, nil
}

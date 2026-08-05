package agenthold

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
)

type Gate struct {
	settings  repository.AgentSetting
	proposals repository.AgentProposal
	agents    repository.Agent
}

func New(
	settings repository.AgentSetting,
	proposals repository.AgentProposal,
	agents repository.Agent,
) *Gate {
	return &Gate{settings: settings, proposals: proposals, agents: agents}
}

func (g *Gate) Hold(
	ctx context.Context,
	decision entity.Decision,
	issue entity.Issue,
	action entity.AgentAction,
	change entity.AgentChange,
) (entity.AgentProposal, bool, error) {
	if decision.Actor.Kind != entity.ActorKindAgent || identity.Approved(ctx) {
		return entity.AgentProposal{}, false, nil
	}

	configured, err := g.settings.Settings(ctx, issue.WorkspaceID, issue.TeamID)
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	if !configured.Holds(action) {
		return entity.AgentProposal{}, false, nil
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

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
	settings    repository.AgentSetting
	proposals   repository.AgentProposal
	agents      repository.Agent
	states      repository.WorkflowState
	delegations repository.IssueDelegation
	questions   repository.IssueQuestion
	notify      repository.NotificationEvent
	checks      *checkgate.Gate
}

func New(
	settings repository.AgentSetting,
	proposals repository.AgentProposal,
	agents repository.Agent,
	states repository.WorkflowState,
	delegations repository.IssueDelegation,
	questions repository.IssueQuestion,
	notify repository.NotificationEvent,
	checks *checkgate.Gate,
) *Gate {
	return &Gate{
		settings:    settings,
		proposals:   proposals,
		agents:      agents,
		states:      states,
		delegations: delegations,
		questions:   questions,
		notify:      notify,
		checks:      checks,
	}
}

func (g *Gate) Hold(
	ctx context.Context,
	decision entity.Decision,
	issue entity.Issue,
	actions []entity.AgentAction,
	change entity.AgentChange,
	reasoning entity.AgentReasoning,
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

	completing, err := g.completing(ctx, issue, change)
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	unanswered, err := g.unanswered(ctx, issue, completing)
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	if len(unanswered) > 0 {
		action, policy = entity.AgentActionStateChange, entity.AgentHoldAlways
	}

	switch policy {
	case entity.AgentHoldNever:
		return entity.AgentProposal{}, false, nil

	case entity.AgentHoldUnlessProven:
		waits, err := g.unproven(ctx, issue, completing)
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
		Reasoning:   reasoning,
	})
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	if err := g.announce(ctx, decision, issue); err != nil {
		return entity.AgentProposal{}, false, err
	}

	return held, true, nil
}

func (g *Gate) announce(ctx context.Context, decision entity.Decision, issue entity.Issue) error {
	return g.notify.Record(ctx, entity.NotificationEvent{
		WorkspaceID: issue.WorkspaceID,
		Subject:     entity.NotifyIssue(issue.ID),
		Kind:        entity.NotificationKindApprovalWaiting,
		Actor:       decision.Actor.AccountID,
		ActorKind:   decision.Actor.Kind,
		Target:      g.awaitedBy(ctx, issue),
	})
}

func (g *Gate) awaitedBy(ctx context.Context, issue entity.Issue) uuid.UUID {
	delegation, err := g.delegations.Open(ctx, issue.WorkspaceID, issue.ID)
	if err != nil {
		return uuid.Nil
	}

	return delegation.DelegatedByAccountID
}

func (g *Gate) HoldCreation(
	ctx context.Context,
	decision entity.Decision,
	workspaceID, teamID uuid.UUID,
	change entity.AgentChange,
	reasoning entity.AgentReasoning,
) (entity.AgentProposal, bool, error) {
	if decision.Actor.Kind != entity.ActorKindAgent || identity.Approved(ctx) {
		return entity.AgentProposal{}, false, nil
	}

	configured, err := g.settings.Settings(ctx, workspaceID, teamID)
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	if configured.Holds(entity.AgentActionIssueCreate) == entity.AgentHoldNever {
		return entity.AgentProposal{}, false, nil
	}

	agent, err := g.agents.GetByAccountID(ctx, decision.Actor.AccountID)
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	held, err := g.proposals.Create(ctx, entity.AgentProposal{
		WorkspaceID: workspaceID,
		AgentID:     agent.ID,
		TeamID:      teamID,
		Action:      entity.AgentActionIssueCreate,
		Change:      change,
		Reasoning:   reasoning,
	})
	if err != nil {
		return entity.AgentProposal{}, false, err
	}

	return held, true, nil
}

func (g *Gate) unproven(ctx context.Context, issue entity.Issue, completing bool) (bool, error) {
	if !completing {
		return false, nil
	}

	blocking, err := g.checks.Blocking(ctx, issue.WorkspaceID, issue.ID)
	if err != nil {
		return false, err
	}

	return len(blocking) > 0, nil
}

func (g *Gate) unanswered(
	ctx context.Context,
	issue entity.Issue,
	completing bool,
) ([]entity.IssueQuestion, error) {
	if !completing {
		return nil, nil
	}

	_, unratified, err := g.checks.Obstructing(ctx, issue.WorkspaceID, issue.ID)
	if err != nil || len(unratified) > 0 {
		return nil, err
	}

	asked, err := g.questions.ListByIssue(ctx, issue.WorkspaceID, issue.ID)
	if err != nil {
		return nil, err
	}

	return entity.UnansweredQuestions(asked), nil
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

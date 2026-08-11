package agent

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (s *agentsService) Settings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.AgentSettings, error) {
	if _, err := s.scopedTeam(ctx, workspaceID, teamID, entity.ActionRead); err != nil {
		return entity.AgentSettings{}, err
	}

	return s.settings.Settings(ctx, workspaceID, teamID)
}

func (s *agentsService) Configure(
	ctx context.Context,
	input service.ConfigureAgentInput,
) (entity.AgentSettings, error) {
	if _, err := s.scopedTeam(ctx, input.WorkspaceID, input.TeamID, entity.ActionManage); err != nil {
		return entity.AgentSettings{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateAgentHold("holdComments", input.HoldComments, entity.AgentActionComment),
		entity.ValidateAgentHold(
			"holdStateChanges", input.HoldStateChanges, entity.AgentActionStateChange,
		),
		entity.ValidateAgentHold("holdIssueEdits", input.HoldIssueEdits, entity.AgentActionIssueEdit),
		entity.ValidateAgentHold(
			"holdIssueCreation", input.HoldIssueCreation, entity.AgentActionIssueCreate,
		),
	); err != nil {
		return entity.AgentSettings{}, err
	}

	return s.settings.Upsert(ctx, entity.AgentSettings{
		TeamID:            input.TeamID,
		WorkspaceID:       input.WorkspaceID,
		HoldComments:      input.HoldComments,
		HoldStateChanges:  input.HoldStateChanges,
		HoldIssueEdits:    input.HoldIssueEdits,
		HoldIssueCreation: input.HoldIssueCreation,
	})
}

func (s *agentsService) scopedTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	action entity.Action,
) (entity.Team, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Team{}, err
	}

	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		return entity.Team{}, err
	}

	if team.WorkspaceID != workspaceID || !decision.Scope.Covers(team.ID) {
		return entity.Team{}, entity.ErrTeamNotFound
	}

	return team, nil
}

func (s *agentsService) Waiting(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]service.WaitingProposal, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return nil, err
	}

	waiting, err := s.proposals.ListWaiting(ctx, workspaceID, entity.ActivityPageMaxSize)
	if err != nil {
		return nil, err
	}

	reachable := make([]service.WaitingProposal, 0, len(waiting))

	for _, proposal := range waiting {
		if !decision.Scope.Covers(proposal.TeamID) {
			continue
		}

		reachable = append(reachable, s.context(ctx, proposal))
	}

	return reachable, nil
}

func (s *agentsService) context(
	ctx context.Context,
	proposal entity.AgentProposal,
) service.WaitingProposal {
	waiting := service.WaitingProposal{Proposal: proposal}

	if team, err := s.teams.GetByID(ctx, proposal.TeamID); err == nil {
		waiting.Team = team
	}

	if proposal.IssueID == uuid.Nil {
		return waiting
	}

	issue, err := s.issues.Get(ctx, proposal.WorkspaceID, proposal.IssueID)
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"reading the issue a held proposal belongs to failed, so the approver sees less of it",
			"proposal_id", proposal.ID.String(),
			"error", err.Error(),
		)

		return waiting
	}

	waiting.Issue = issue

	checks, err := s.checks.List(ctx, proposal.WorkspaceID, proposal.IssueID)
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"reading the checks on a held proposal's issue failed, so the approver sees less of it",
			"proposal_id", proposal.ID.String(),
			"error", err.Error(),
		)

		return waiting
	}

	waiting.Checks = checks
	waiting.Proposed = proposedChecks(checks, proposal.Change.CheckIDs)
	waiting.State = s.targetState(ctx, issue, proposal)

	return waiting
}

func (s *agentsService) targetState(
	ctx context.Context,
	issue entity.Issue,
	proposal entity.AgentProposal,
) entity.WorkflowState {
	if proposal.Change.StateID == nil || *proposal.Change.StateID == uuid.Nil {
		return entity.WorkflowState{}
	}

	states, err := s.states.ListByTeamID(ctx, issue.TeamID)
	if err != nil {
		return entity.WorkflowState{}
	}

	for _, state := range states {
		if state.ID == *proposal.Change.StateID {
			return state
		}
	}

	return entity.WorkflowState{}
}

func proposedChecks(checks service.IssueChecks, ids []uuid.UUID) []entity.Check {
	if len(ids) == 0 {
		return nil
	}

	proposed := make([]entity.Check, 0, len(ids))

	for _, report := range checks.Reports {
		if slices.Contains(ids, report.Check.ID) {
			proposed = append(proposed, report.Check)
		}
	}

	return proposed
}

func (s *agentsService) Approve(
	ctx context.Context,
	workspaceID, proposalID uuid.UUID,
) (entity.AgentProposal, error) {
	decision, proposal, err := s.decidable(ctx, workspaceID, proposalID)
	if err != nil {
		return entity.AgentProposal{}, err
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, proposal.AgentID)
	if err != nil {
		return entity.AgentProposal{}, err
	}

	if agent.Disabled() {
		return entity.AgentProposal{}, entity.ErrAgentDisabled
	}

	now := time.Now().UTC()

	if err := s.proposals.Settle(
		ctx, proposal.ID, entity.AgentProposalApplied, decision.Actor.AccountID, now, "",
	); err != nil {
		return entity.AgentProposal{}, err
	}

	if err := s.apply(ctx, agent, proposal, decision.Actor.AccountID); err != nil {
		if settled := s.proposals.Settle(
			ctx, proposal.ID, entity.AgentProposalFailed, decision.Actor.AccountID, now, err.Error(),
		); settled != nil && !errors.Is(settled, entity.ErrAgentProposalSettled) {
			return entity.AgentProposal{}, settled
		}

		return s.proposals.GetByID(ctx, workspaceID, proposal.ID)
	}

	s.recordDecision(ctx, workspaceID, proposal, entity.AgentProposalApplied)

	return s.proposals.GetByID(ctx, workspaceID, proposal.ID)
}

func (s *agentsService) Reject(
	ctx context.Context,
	workspaceID, proposalID uuid.UUID,
) (entity.AgentProposal, error) {
	decision, proposal, err := s.decidable(ctx, workspaceID, proposalID)
	if err != nil {
		return entity.AgentProposal{}, err
	}

	if err := s.proposals.Settle(
		ctx, proposal.ID, entity.AgentProposalRejected,
		decision.Actor.AccountID, time.Now().UTC(), "",
	); err != nil {
		return entity.AgentProposal{}, err
	}

	s.recordDecision(ctx, workspaceID, proposal, entity.AgentProposalRejected)

	return s.proposals.GetByID(ctx, workspaceID, proposal.ID)
}

func (s *agentsService) recordDecision(
	ctx context.Context,
	workspaceID uuid.UUID,
	proposal entity.AgentProposal,
	outcome entity.AgentProposalStatus,
) {
	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditAgentProposal,
		ResourceKind: string(entity.ResourceAgent),
		ResourceID:   proposal.AgentID,
		Detail: map[string]string{
			"proposal_id": proposal.ID.String(),
			"outcome":     string(outcome),
		},
	})
}

func (s *agentsService) decidable(
	ctx context.Context,
	workspaceID, proposalID uuid.UUID,
) (entity.Decision, entity.AgentProposal, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, entity.AgentProposal{}, err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.Decision{}, entity.AgentProposal{}, entity.ErrAPITokenMintForbidden
	}

	proposal, err := s.proposals.GetByID(ctx, workspaceID, proposalID)
	if err != nil {
		return entity.Decision{}, entity.AgentProposal{}, err
	}

	if !decision.Scope.Covers(proposal.TeamID) {
		return entity.Decision{}, entity.AgentProposal{}, entity.ErrAgentProposalNotFound
	}

	if proposal.Status.Settled() {
		return entity.Decision{}, entity.AgentProposal{}, entity.ErrAgentProposalSettled
	}

	return decision, proposal, nil
}

func (s *agentsService) apply(
	ctx context.Context,
	agent entity.Agent,
	proposal entity.AgentProposal,
	approver uuid.UUID,
) error {
	acting := identity.WithApproval(identity.WithActor(ctx, entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      agent.AccountID,
		AgentID:        &agent.ID,
		AgentAllowance: agent.Allowance(),
		OwnerAccountID: agent.OwnerAccountID,
		Grants: entity.APITokenGrants{{
			WorkspaceID: proposal.WorkspaceID,
			AllTeams:    true,
		}},
		Scopes: proposal.Action.Scopes(),
	}), approver)

	switch proposal.Action {
	case entity.AgentActionComment:
		_, err := s.comments.Post(
			acting,
			proposal.WorkspaceID,
			proposal.IssueID,
			service.PostCommentInput{Body: proposal.Change.Body},
		)

		return err
	case entity.AgentActionCheckSet:
		return s.approveChecks(ctx, proposal)
	case entity.AgentActionIssueCreate:
		_, err := s.issues.Create(acting, creationFrom(proposal))

		return err
	case entity.AgentActionStateChange, entity.AgentActionIssueEdit:
		_, err := s.issues.Update(
			acting,
			proposal.WorkspaceID,
			proposal.IssueID,
			service.UpdateIssueInput{
				ExpectedVersion: proposal.Change.ExpectedVersion,
				StateID:         proposal.Change.StateID,
				Title:           proposal.Change.Title,
				Description:     proposal.Change.Description,
				Priority:        proposal.Change.Priority,
				AssigneeID:      proposal.Change.AssigneeID,
				Estimate:        proposal.Change.Estimate,
				DueOn:           proposal.Change.DueOn,
				CycleID:         proposal.Change.CycleID,
				ProjectID:       proposal.Change.ProjectID,
				Clear:           proposal.Change.Clear,
			},
		)

		return err
	default:
		return entity.ErrAgentProposalNotFound
	}
}

func creationFrom(proposal entity.AgentProposal) service.CreateIssueInput {
	change := proposal.Change

	input := service.CreateIssueInput{
		WorkspaceID: proposal.WorkspaceID,
		TeamID:      proposal.TeamID,
		LabelIDs:    change.LabelIDs,
	}

	if change.Title != nil {
		input.Title = *change.Title
	}

	if change.Description != nil {
		input.Description = *change.Description
	}

	if change.Priority != nil {
		input.Priority = *change.Priority
	}

	if change.Estimate != nil {
		input.Estimate = *change.Estimate
	}

	if change.DueOn != nil {
		input.DueOn = *change.DueOn
	}

	for target, source := range map[*uuid.UUID]*uuid.UUID{
		&input.StateID:           change.StateID,
		&input.AssigneeAccountID: change.AssigneeID,
		&input.CycleID:           change.CycleID,
		&input.ProjectID:         change.ProjectID,
	} {
		if source != nil {
			*target = *source
		}
	}

	return input
}

func (s *agentsService) approveChecks(ctx context.Context, proposal entity.AgentProposal) error {
	for _, checkID := range proposal.Change.CheckIDs {
		if _, err := s.checks.Decide(
			ctx,
			proposal.WorkspaceID,
			proposal.IssueID,
			checkID,
			service.DecideCheckInput{Approval: entity.CheckApprovalApproved},
		); err != nil && !errors.Is(err, entity.ErrCheckDecided) {
			return err
		}
	}

	return nil
}

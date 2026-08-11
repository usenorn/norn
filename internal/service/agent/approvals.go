package agent

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
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
	); err != nil {
		return entity.AgentSettings{}, err
	}

	return s.settings.Upsert(ctx, entity.AgentSettings{
		TeamID:           input.TeamID,
		WorkspaceID:      input.WorkspaceID,
		HoldComments:     input.HoldComments,
		HoldStateChanges: input.HoldStateChanges,
		HoldIssueEdits:   input.HoldIssueEdits,
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
) ([]entity.AgentProposal, error) {
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

	reachable := make([]entity.AgentProposal, 0, len(waiting))

	for _, proposal := range waiting {
		if decision.Scope.Covers(proposal.TeamID) {
			reachable = append(reachable, proposal)
		}
	}

	return reachable, nil
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

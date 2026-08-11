package issue

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func set(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}

	return &id
}

func (s *issuesService) heldCreation(
	ctx context.Context,
	decision entity.Decision,
	input service.CreateIssueInput,
) error {
	change := entity.AgentChange{
		Title:      &input.Title,
		LabelIDs:   input.LabelIDs,
		StateID:    set(input.StateID),
		AssigneeID: set(input.AssigneeAccountID),
		CycleID:    set(input.CycleID),
		ProjectID:  set(input.ProjectID),
	}

	if input.Description != "" {
		change.Description = &input.Description
	}

	if input.Priority != "" {
		change.Priority = &input.Priority
	}

	if input.Estimate != 0 {
		change.Estimate = &input.Estimate
	}

	if input.DueOn != "" {
		change.DueOn = &input.DueOn
	}

	proposal, held, err := s.gate.HoldCreation(
		ctx, decision, input.WorkspaceID, input.TeamID, change, input.Reasoning,
	)
	if err != nil {
		return err
	}

	if held {
		return entity.AgentActionHeldError{ProposalID: proposal.ID}
	}

	return nil
}

func (s *issuesService) held(
	ctx context.Context,
	decision entity.Decision,
	workspaceID, issueID uuid.UUID,
	input service.UpdateIssueInput,
	change entity.IssueChange,
) (bool, error) {
	if decision.Actor.Kind != entity.ActorKindAgent {
		return false, nil
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return false, err
	}

	proposal, held, err := s.gate.Hold(
		ctx,
		decision,
		issue,
		change.AgentActions(issue),
		entity.AgentChange{
			StateID:     input.StateID,
			Title:       input.Title,
			Description: input.Description,
			Priority:    input.Priority,
			AssigneeID:  input.AssigneeID,
			Estimate:    input.Estimate,
			DueOn:       input.DueOn,
			CycleID:     input.CycleID,
			ProjectID:   input.ProjectID,
			Clear:       input.Clear,
		},
		input.Reasoning,
	)
	if err != nil {
		return false, err
	}

	if held {
		return true, entity.AgentActionHeldError{ProposalID: proposal.ID}
	}

	return false, nil
}

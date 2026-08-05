package issue

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *issuesService) held(
	ctx context.Context,
	decision entity.Decision,
	workspaceID, issueID uuid.UUID,
	input service.UpdateIssueInput,
	touched []string,
) (bool, error) {
	if decision.Actor.Kind != entity.ActorKindAgent {
		return false, nil
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return false, err
	}

	action := entity.AgentActionIssueEdit
	if slices.Contains(touched, entity.IssueFieldState) && len(touched) == 1 {
		action = entity.AgentActionStateChange
	}

	proposal, held, err := s.gate.Hold(ctx, decision, issue, action, entity.AgentChange{
		StateID:    input.StateID,
		Title:      input.Title,
		Priority:   input.Priority,
		AssigneeID: input.AssigneeID,
	})
	if err != nil {
		return false, err
	}

	if held {
		return true, entity.AgentActionHeldError{ProposalID: proposal.ID}
	}

	return false, nil
}

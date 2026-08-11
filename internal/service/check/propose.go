package check

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func (s *checksService) propose(
	ctx context.Context,
	decision entity.Decision,
	issue entity.Issue,
	added []entity.Check,
	reasoning entity.AgentReasoning,
) error {
	if decision.Actor.Kind != entity.ActorKindAgent || len(added) == 0 {
		return nil
	}

	agent, err := s.agents.GetByAccountID(ctx, decision.Actor.AccountID)
	if err != nil {
		return err
	}

	ids := make([]uuid.UUID, 0, len(added))

	for _, check := range added {
		ids = append(ids, check.ID)
	}

	_, err = s.proposals.Create(ctx, entity.AgentProposal{
		WorkspaceID: issue.WorkspaceID,
		AgentID:     agent.ID,
		IssueID:     issue.ID,
		TeamID:      issue.TeamID,
		Action:      entity.AgentActionCheckSet,
		Change:      entity.AgentChange{ExpectedVersion: issue.Version, CheckIDs: ids},
		Reasoning:   reasoning,
	})

	return err
}

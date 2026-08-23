package changeset

import (
	"context"
	"encoding/json"

	"github.com/usenorn/norn/internal/entity"
)

func (s *changeSetsService) announce(ctx context.Context, execution entity.Execution) {
	stored, err := s.changesets.Get(ctx, execution.ID)
	if err != nil {
		return
	}

	payload, err := json.Marshal(stored)
	if err != nil {
		return
	}

	s.events.Publish(ctx, entity.Event{
		WorkspaceID: execution.WorkspaceID,
		Kind:        entity.EventExecutionChangeSet,
		TeamID:      execution.TeamID,
		SubjectID:   execution.IssueID,
		IssueID:     execution.IssueID,
		Payload:     payload,
	})
}

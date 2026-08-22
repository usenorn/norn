package execution

import (
	"context"
	"encoding/json"

	"github.com/usenorn/norn/internal/entity"
)

func (s *executionsService) broadcast(ctx context.Context, execution entity.Execution) {
	payload, err := json.Marshal(execution)
	if err != nil {
		return
	}

	s.events.Publish(ctx, entity.Event{
		WorkspaceID: execution.WorkspaceID,
		Kind:        entity.EventExecutionUpdated,
		TeamID:      execution.TeamID,
		SubjectID:   execution.IssueID,
		IssueID:     execution.IssueID,
		Payload:     payload,
	})
}

func (s *executionsService) announce(
	ctx context.Context,
	execution entity.Execution,
	event entity.ExecutionEvent,
) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.events.Publish(ctx, entity.Event{
		WorkspaceID: execution.WorkspaceID,
		Kind:        entity.EventExecutionEvent,
		TeamID:      execution.TeamID,
		SubjectID:   execution.IssueID,
		IssueID:     execution.IssueID,
		Payload:     payload,
	})
}

package issue

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

// broadcast queues the change for connected clients once the transaction it belongs to has
// committed. Registering inside the transaction and running after it is what keeps a rolled-back
// change from being announced as though it happened.
func (s *issuesService) broadcast(
	ctx context.Context,
	kind entity.EventKind,
	issue entity.Issue,
	actor uuid.UUID,
) {
	payload, err := json.Marshal(issue)
	if err != nil {
		return
	}

	event := entity.Event{
		WorkspaceID: issue.WorkspaceID,
		Kind:        kind,
		TeamID:      issue.TeamID,
		SubjectID:   issue.ID,
		IssueID:     issue.ID,
		ActorID:     actor,
		Payload:     payload,
	}

	postgres.AfterCommit(ctx, func(ctx context.Context) {
		s.events.Publish(ctx, event)
	})
}

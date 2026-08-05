package issue

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

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

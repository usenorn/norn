package issuecomment

import (
	"context"
	"encoding/json"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

// broadcast queues the comment for connected clients once the transaction has committed. It
// carries the issue's team because that is what decides who may be shown it, and asking the
// database for it per recipient would put a query on the delivery path.
func (s *issueCommentsService) broadcast(
	ctx context.Context,
	kind entity.EventKind,
	issue entity.Issue,
	comment entity.IssueComment,
) {
	payload, err := json.Marshal(comment)
	if err != nil {
		return
	}

	event := entity.Event{
		WorkspaceID: issue.WorkspaceID,
		Kind:        kind,
		TeamID:      issue.TeamID,
		SubjectID:   comment.ID,
		IssueID:     issue.ID,
		ActorID:     comment.AuthorAccountID,
		Payload:     payload,
	}

	postgres.AfterCommit(ctx, func(ctx context.Context) {
		s.events.Publish(ctx, event)
	})
}

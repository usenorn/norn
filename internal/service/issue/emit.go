package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *issuesService) emit(
	ctx context.Context,
	event entity.WebhookEvent,
	issue entity.Issue,
	decision entity.Decision,
) error {
	body, err := json.Marshal(service.WebhookIssue(issue))
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", event, err)
	}

	return s.emitter.Emit(ctx, entity.WebhookOutboxEntry{
		WorkspaceID: issue.WorkspaceID,
		Event:       event,
		SubjectKind: string(entity.ResourceIssue),
		SubjectID:   issue.ID,
		TeamID:      issue.TeamID,
		ActorID:     decision.Actor.AccountID,
		ActorKind:   decision.Actor.Kind,
		Body:        body,
	})
}

package project

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *projectsService) emit(
	ctx context.Context,
	event entity.WebhookEvent,
	project entity.Project,
	decision entity.Decision,
) error {
	body, err := json.Marshal(service.WebhookProject(project))
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", event, err)
	}

	return s.emitter.Emit(ctx, entity.WebhookOutboxEntry{
		WorkspaceID: project.WorkspaceID,
		Event:       event,
		SubjectKind: string(entity.ResourceProject),
		SubjectID:   project.ID,
		ActorID:     decision.Actor.AccountID,
		ActorKind:   decision.Actor.Kind,
		Body:        body,
	})
}

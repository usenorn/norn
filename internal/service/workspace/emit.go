package workspace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/service"
)

func (s *workspacesService) emit(
	ctx context.Context,
	event entity.WebhookEvent,
	membership entity.Membership,
	decision entity.Decision,
) error {
	body, err := json.Marshal(service.WebhookMembership(membership))
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", event, err)
	}

	return s.emitter.Emit(ctx, entity.WebhookOutboxEntry{
		WorkspaceID: membership.WorkspaceID,
		Event:       event,
		SubjectKind: string(entity.ResourceMembership),
		SubjectID:   membership.AccountID,
		ActorID:     decision.Actor.AccountID,
		ActorKind:   decision.Actor.Kind,
		Body:        body,
	})
}

func (s *workspacesService) rescope(ctx context.Context, workspaceID, accountID uuid.UUID) {
	postgres.AfterCommit(ctx, func(ctx context.Context) {
		s.events.Publish(ctx, entity.Event{
			WorkspaceID: workspaceID,
			Kind:        entity.EventMembershipChanged,
			AccountID:   accountID,
		})
	})
}

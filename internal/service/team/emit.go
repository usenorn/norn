package team

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/service"
)

func (s *teamsService) emit(
	ctx context.Context,
	event entity.WebhookEvent,
	accountID uuid.UUID,
	team entity.Team,
	decision entity.Decision,
) error {
	body, err := json.Marshal(service.WebhookTeamMembership(accountID, team))
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", event, err)
	}

	return s.emitter.Emit(ctx, entity.WebhookOutboxEntry{
		WorkspaceID: team.WorkspaceID,
		Event:       event,
		SubjectKind: string(entity.ResourceTeamMembership),
		SubjectID:   accountID,
		TeamID:      team.ID,
		ActorID:     decision.Actor.AccountID,
		ActorKind:   decision.Actor.Kind,
		Body:        body,
	})
}

func (s *teamsService) rescope(ctx context.Context, workspaceID, accountID uuid.UUID) {
	postgres.AfterCommit(ctx, func(ctx context.Context) {
		s.events.Publish(ctx, entity.Event{
			WorkspaceID: workspaceID,
			Kind:        entity.EventMembershipChanged,
			AccountID:   accountID,
		})
	})
}

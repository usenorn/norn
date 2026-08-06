package directory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/service"
)

func (s *directoriesService) emit(
	ctx context.Context,
	event entity.WebhookEvent,
	membership entity.Membership,
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
		ActorKind:   entity.ActorKindSystem,
		Body:        body,
	})
}

func (s *directoriesService) emitTeamMembership(
	ctx context.Context,
	event entity.WebhookEvent,
	accountID uuid.UUID,
	team entity.Team,
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
		ActorKind:   entity.ActorKindSystem,
		Body:        body,
	})
}

func (s *directoriesService) mappedTeam(ctx context.Context, teamID uuid.UUID) (entity.Team, bool) {
	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"reading the team a directory group maps to failed",
			"team_id", teamID.String(),
			"error", err.Error(),
		)

		return entity.Team{}, false
	}

	return team, true
}

func (s *directoriesService) rescope(ctx context.Context, workspaceID, accountID uuid.UUID) {
	postgres.AfterCommit(ctx, func(ctx context.Context) {
		s.events.Publish(ctx, entity.Event{
			WorkspaceID: workspaceID,
			Kind:        entity.EventMembershipChanged,
			AccountID:   accountID,
		})
	})
}

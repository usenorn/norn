package cycle

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *cyclesService) emit(
	ctx context.Context,
	event entity.WebhookEvent,
	cycle entity.Cycle,
	decision entity.Decision,
) error {
	body, err := json.Marshal(service.WebhookCycle(cycle))
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", event, err)
	}

	return s.emitter.Emit(ctx, entity.WebhookOutboxEntry{
		WorkspaceID: cycle.WorkspaceID,
		Event:       event,
		SubjectKind: string(entity.ResourceCycle),
		SubjectID:   cycle.ID,
		TeamID:      cycle.TeamID,
		ActorID:     decision.Actor.AccountID,
		ActorKind:   decision.Actor.Kind,
		Body:        body,
	})
}

func (s *cyclesService) emitCadence(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	upcoming []service.CycleView,
	decision entity.Decision,
) error {
	next := entity.Cycle{WorkspaceID: workspaceID, TeamID: teamID}
	if len(upcoming) > 0 {
		next = upcoming[0].Cycle
	}

	body, err := json.Marshal(service.WebhookCycle(next))
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", entity.WebhookCycleCadenceChanged, err)
	}

	return s.emitter.Emit(ctx, entity.WebhookOutboxEntry{
		WorkspaceID: workspaceID,
		Event:       entity.WebhookCycleCadenceChanged,
		SubjectKind: string(entity.ResourceCycle),
		SubjectID:   uuid.Nil,
		TeamID:      teamID,
		ActorID:     decision.Actor.AccountID,
		ActorKind:   decision.Actor.Kind,
		Body:        body,
	})
}

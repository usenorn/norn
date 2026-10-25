package memoryevidence

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type checks struct {
	memories   repository.Memory
	notify     repository.NotificationEvent
	jobs       repository.JobProducer
	transactor repository.Transactor
}

func New(
	memories repository.Memory,
	notify repository.NotificationEvent,
	jobs repository.JobProducer,
	transactor repository.Transactor,
) service.MemoryEvidenceChecks {
	return &checks{memories: memories, notify: notify, jobs: jobs, transactor: transactor}
}

func (s *checks) Schedule(ctx context.Context, workspaceID uuid.UUID) {
	postgres.AfterCommit(ctx, func(ctx context.Context) {
		payload := entity.MemoryEvidenceCheckPayload{WorkspaceID: workspaceID}
		if err := s.jobs.EnqueueMemoryEvidenceCheck(ctx, payload); err != nil {
			logging.From(ctx).WarnContext(
				ctx,
				"scheduling a memory evidence check failed, so the scheduled sweep will carry this change",
				"workspace_id", workspaceID.String(),
				"error", err.Error(),
			)
		}
	})
}

func (s *checks) Check(ctx context.Context, workspaceID uuid.UUID) error {
	memoryIDs, err := s.memories.Unalerted(ctx, workspaceID)
	if err != nil {
		return err
	}

	for _, memoryID := range memoryIDs {
		if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
			return s.alert(ctx, workspaceID, memoryID)
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *checks) alert(ctx context.Context, workspaceID, memoryID uuid.UUID) error {
	memory, err := s.memories.LockByID(ctx, workspaceID, memoryID)
	if errors.Is(err, entity.ErrMemoryNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	if memory.Status.Terminal() {
		return nil
	}

	alerted := false

	for position, evidence := range memory.Evidence {
		if !evidence.Unalerted() {
			continue
		}

		if err := s.memories.Alerted(ctx, memory.ID, position, evidence.State); err != nil {
			return err
		}

		alerted = true
	}

	if !alerted {
		return nil
	}

	for _, editor := range memory.Editors() {
		if err := s.notify.Record(ctx, entity.NotificationEvent{
			WorkspaceID: workspaceID,
			Subject:     entity.NotifyMemory(memory.ID),
			Kind:        entity.NotificationKindMemoryEvidenceChanged,
			ActorKind:   entity.ActorKindSystem,
			Target:      editor,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *checks) Sweep(ctx context.Context) error {
	workspaceIDs, err := s.memories.UnalertedWorkspaces(ctx)
	if err != nil {
		return err
	}

	var failures []error

	for _, workspaceID := range workspaceIDs {
		if err := s.Check(ctx, workspaceID); err != nil {
			failures = append(failures, err)
		}
	}

	return errors.Join(failures...)
}

package jobs

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type jobsService struct {
	inspector repository.JobInspector
}

func New(inspector repository.JobInspector) service.Jobs {
	return &jobsService{inspector: inspector}
}

func (s *jobsService) Queues(ctx context.Context) ([]entity.QueueStat, error) {
	return s.inspector.Queues(ctx)
}

func (s *jobsService) List(ctx context.Context, queue string, state entity.TaskState) ([]entity.TaskSummary, error) {
	if !state.Valid() {
		return nil, entity.NewValidationError(entity.FieldError{
			Field: "state",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	return s.inspector.List(ctx, queue, state)
}

func (s *jobsService) Run(ctx context.Context, queue, taskID string) error {
	return s.inspector.Run(ctx, queue, taskID)
}

func (s *jobsService) Archive(ctx context.Context, queue, taskID string) error {
	return s.inspector.Archive(ctx, queue, taskID)
}

func (s *jobsService) Delete(ctx context.Context, queue, taskID string) error {
	return s.inspector.Delete(ctx, queue, taskID)
}

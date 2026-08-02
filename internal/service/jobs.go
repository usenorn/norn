package service

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=jobs.go -destination=jobs/mock_jobs.go -package=jobs -mock_names=Jobs=MockJobs

type Jobs interface {
	Queues(ctx context.Context) ([]entity.QueueStat, error)
	List(ctx context.Context, queue string, state entity.TaskState) ([]entity.TaskSummary, error)
	Run(ctx context.Context, queue, taskID string) error
	Archive(ctx context.Context, queue, taskID string) error
	Delete(ctx context.Context, queue, taskID string) error
}

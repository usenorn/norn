package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type JobsAdmin struct {
	jobs service.Jobs
	out  io.Writer
}

func NewJobsAdmin(jobs service.Jobs) *JobsAdmin {
	return &JobsAdmin{jobs: jobs, out: os.Stdout}
}

func (a *JobsAdmin) Queues(ctx context.Context) error {
	stats, err := a.jobs.Queues(ctx)
	if err != nil {
		return err
	}

	return a.write(stats)
}

func (a *JobsAdmin) List(ctx context.Context, queue, state string) error {
	tasks, err := a.jobs.List(ctx, queue, entity.TaskState(state))
	if err != nil {
		return err
	}

	return a.write(tasks)
}

func (a *JobsAdmin) Run(ctx context.Context, queue, taskID string) error {
	return a.jobs.Run(ctx, queue, taskID)
}

func (a *JobsAdmin) Archive(ctx context.Context, queue, taskID string) error {
	return a.jobs.Archive(ctx, queue, taskID)
}

func (a *JobsAdmin) Delete(ctx context.Context, queue, taskID string) error {
	return a.jobs.Delete(ctx, queue, taskID)
}

func (a *JobsAdmin) write(payload any) error {
	encoder := json.NewEncoder(a.out)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("write jobs output: %w", err)
	}

	return nil
}

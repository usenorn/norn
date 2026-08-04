package jobqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/taskqueue"
	"github.com/usenorn/norn/internal/repository"
)

type Client struct {
	producer  *taskqueue.Client
	inspector *taskqueue.Inspector
	maxRetry  int
}

func NewClient(producer *taskqueue.Client, inspector *taskqueue.Inspector, cfg config.Asynq) *Client {
	return &Client{producer: producer, inspector: inspector, maxRetry: cfg.MaxRetry}
}

func AsProducer(client *Client) repository.JobProducer {
	return client
}

func AsInspector(client *Client) repository.JobInspector {
	return client
}

func (c *Client) EnqueueEmailChangeConfirmation(ctx context.Context, payload entity.EmailChangeConfirmationPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode email change confirmation payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeEmailChangeConfirmation, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueMail),
		asynq.MaxRetry(c.maxRetry),
	); err != nil {
		return fmt.Errorf("enqueue email change confirmation: %w", err)
	}

	return nil
}

func (c *Client) EnqueueSignUpVerification(ctx context.Context, payload entity.SignUpVerificationPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode sign-up verification payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeSignUpVerification, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueMail),
		asynq.MaxRetry(c.maxRetry),
	); err != nil {
		return fmt.Errorf("enqueue sign-up verification: %w", err)
	}

	return nil
}

func (c *Client) EnqueuePasswordReset(ctx context.Context, payload entity.PasswordResetPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode password reset payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypePasswordReset, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueMail),
		asynq.MaxRetry(c.maxRetry),
	); err != nil {
		return fmt.Errorf("enqueue password reset: %w", err)
	}

	return nil
}

func (c *Client) EnqueuePasswordResetSSONotice(ctx context.Context, payload entity.PasswordResetSSONoticePayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode password reset sso notice payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypePasswordResetSSONotice, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueMail),
		asynq.MaxRetry(c.maxRetry),
	); err != nil {
		return fmt.Errorf("enqueue password reset sso notice: %w", err)
	}

	return nil
}

func (c *Client) EnqueueInvitation(ctx context.Context, payload entity.InvitationPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode invitation payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeInvitation, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueMail),
		asynq.MaxRetry(c.maxRetry),
	); err != nil {
		return fmt.Errorf("enqueue invitation: %w", err)
	}

	return nil
}

func (c *Client) EnqueueWorkspacePurge(ctx context.Context, payload entity.WorkspacePurgePayload, processAt time.Time) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode workspace purge payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeWorkspacePurge, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueDefault),
		asynq.MaxRetry(c.maxRetry),
		asynq.ProcessAt(processAt),
	); err != nil {
		return fmt.Errorf("enqueue workspace purge: %w", err)
	}

	return nil
}

func (c *Client) EnqueueIssuePurge(ctx context.Context, payload entity.IssuePurgePayload, processAt time.Time) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode issue purge payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeIssuePurge, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueDefault),
		asynq.MaxRetry(c.maxRetry),
		asynq.ProcessAt(processAt),
	); err != nil {
		return fmt.Errorf("enqueue issue purge: %w", err)
	}

	return nil
}

func (c *Client) Queues(_ context.Context) ([]entity.QueueStat, error) {
	names, err := c.inspector.Queues()
	if err != nil {
		return nil, fmt.Errorf("list queues: %w", err)
	}

	stats := make([]entity.QueueStat, 0, len(names))

	for _, name := range names {
		info, err := c.inspector.GetQueueInfo(name)
		if err != nil {
			return nil, fmt.Errorf("inspect queue %q: %w", name, err)
		}

		stats = append(stats, entity.QueueStat{
			Name:      info.Queue,
			Size:      info.Size,
			Active:    info.Active,
			Pending:   info.Pending,
			Scheduled: info.Scheduled,
			Retry:     info.Retry,
			Archived:  info.Archived,
			Completed: info.Completed,
		})
	}

	return stats, nil
}

func (c *Client) List(_ context.Context, queue string, state entity.TaskState) ([]entity.TaskSummary, error) {
	tasks, err := c.listByState(queue, state)
	if err != nil {
		return nil, err
	}

	summaries := make([]entity.TaskSummary, 0, len(tasks))

	for _, task := range tasks {
		summaries = append(summaries, entity.TaskSummary{
			ID:        task.ID,
			Queue:     task.Queue,
			Type:      task.Type,
			State:     state,
			Retried:   task.Retried,
			MaxRetry:  task.MaxRetry,
			LastError: task.LastErr,
			NextRetry: task.NextProcessAt,
		})
	}

	return summaries, nil
}

func (c *Client) listByState(queue string, state entity.TaskState) ([]*asynq.TaskInfo, error) {
	var (
		tasks []*asynq.TaskInfo
		err   error
	)

	switch state {
	case entity.TaskStateActive:
		tasks, err = c.inspector.ListActiveTasks(queue)
	case entity.TaskStatePending:
		tasks, err = c.inspector.ListPendingTasks(queue)
	case entity.TaskStateScheduled:
		tasks, err = c.inspector.ListScheduledTasks(queue)
	case entity.TaskStateRetry:
		tasks, err = c.inspector.ListRetryTasks(queue)
	case entity.TaskStateArchived:
		tasks, err = c.inspector.ListArchivedTasks(queue)
	case entity.TaskStateCompleted:
		tasks, err = c.inspector.ListCompletedTasks(queue)
	default:
		return nil, fmt.Errorf("unknown task state %q", state)
	}

	if err != nil {
		return nil, fmt.Errorf("list %s tasks in queue %q: %w", state, queue, err)
	}

	return tasks, nil
}

func (c *Client) Run(_ context.Context, queue, taskID string) error {
	return translateTaskError(c.inspector.RunTask(queue, taskID), "run")
}

func (c *Client) Archive(_ context.Context, queue, taskID string) error {
	return translateTaskError(c.inspector.ArchiveTask(queue, taskID), "archive")
}

func (c *Client) Delete(_ context.Context, queue, taskID string) error {
	return translateTaskError(c.inspector.DeleteTask(queue, taskID), "delete")
}

func translateTaskError(err error, action string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, asynq.ErrTaskNotFound), errors.Is(err, asynq.ErrQueueNotFound):
		return entity.ErrTaskNotFound
	default:
		return fmt.Errorf("%s task: %w", action, err)
	}
}

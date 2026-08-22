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
	producer      *taskqueue.Client
	inspector     *taskqueue.Inspector
	maxRetry      int
	importTimeout time.Duration
}

func NewClient(
	producer *taskqueue.Client,
	inspector *taskqueue.Inspector,
	cfg config.Asynq,
	imports config.Imports,
) *Client {
	return &Client{
		producer:      producer,
		inspector:     inspector,
		maxRetry:      cfg.MaxRetry,
		importTimeout: 2 * imports.SliceBudget,
	}
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

func (c *Client) EnqueueSignInCode(ctx context.Context, payload entity.SignInCodePayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode sign-in code payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeSignInCode, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueMail),
		asynq.MaxRetry(c.maxRetry),
	); err != nil {
		return fmt.Errorf("enqueue sign-in code: %w", err)
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

func (c *Client) EnqueueAttachmentReclaim(ctx context.Context) error {
	task := asynq.NewTask(entity.TaskTypeAttachmentReclaim, nil)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueDefault),
		asynq.MaxRetry(c.maxRetry),
		asynq.TaskID(entity.AttachmentReclaimTaskID),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue attachment reclaim: %w", err)
	}

	return nil
}

func (c *Client) EnqueueWebhookFanOut(ctx context.Context) error {
	task := asynq.NewTask(entity.TaskTypeWebhookFanOut, nil)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueWebhook),
		asynq.MaxRetry(c.maxRetry),
		asynq.TaskID(entity.WebhookFanOutTaskID),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue webhook fan-out: %w", err)
	}

	return nil
}

func (c *Client) EnqueueWebhookDeliver(
	ctx context.Context,
	payload entity.WebhookDeliverPayload,
	processAt time.Time,
) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode webhook delivery payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeWebhookDeliver, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueWebhook),
		asynq.MaxRetry(c.maxRetry),
		asynq.ProcessAt(processAt),
		asynq.TaskID(fmt.Sprintf("%s:%d", payload.DeliveryID, payload.Attempt)),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue webhook delivery: %w", err)
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

func (c *Client) EnqueueBulkApply(ctx context.Context, payload entity.BulkApplyPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode bulk apply payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeBulkApply, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueDefault),
		asynq.MaxRetry(c.maxRetry),
		asynq.TaskID(payload.BulkActionID.String()),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue bulk apply: %w", err)
	}

	return nil
}

func (c *Client) EnqueueImportStage(
	ctx context.Context,
	payload entity.ImportStagePayload,
	processAt time.Time,
) error {
	return c.enqueueImportSlice(
		ctx, entity.TaskTypeImportStage, "import stage", payload,
		fmt.Sprintf("import-stage:%s:%d", payload.ImportRunID, payload.Attempt), processAt,
	)
}

func (c *Client) EnqueueImportExecute(
	ctx context.Context,
	payload entity.ImportExecutePayload,
	processAt time.Time,
) error {
	return c.enqueueImportSlice(
		ctx, entity.TaskTypeImportExecute, "import execute", payload,
		fmt.Sprintf("import-execute:%s:%d", payload.ImportRunID, payload.Attempt), processAt,
	)
}

func (c *Client) EnqueueImportRevert(
	ctx context.Context,
	payload entity.ImportRevertPayload,
	processAt time.Time,
) error {
	return c.enqueueImportSlice(
		ctx, entity.TaskTypeImportRevert, "import revert", payload,
		fmt.Sprintf("import-revert:%s:%d", payload.ImportRunID, payload.Attempt), processAt,
	)
}

func (c *Client) enqueueImportSlice(
	ctx context.Context,
	taskType, action string,
	payload any,
	taskID string,
	processAt time.Time,
) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", action, err)
	}

	task := asynq.NewTask(taskType, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueImport),
		asynq.MaxRetry(c.maxRetry),
		asynq.ProcessAt(processAt),
		asynq.TaskID(taskID),
		asynq.Timeout(c.importTimeout),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue %s: %w", action, err)
	}

	return nil
}

func (c *Client) EnqueueImportRescue(ctx context.Context) error {
	task := asynq.NewTask(entity.TaskTypeImportRescue, nil)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueImport),
		asynq.MaxRetry(c.maxRetry),
		asynq.TaskID(entity.ImportRescueTaskID),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue import rescue: %w", err)
	}

	return nil
}

// EnqueueSCMDelivery keys a task by the delivery and the attempt, so a forge redelivering
// the same event while the first attempt is still queued adds nothing, and a genuine retry
// after a rate limit is a different task rather than a conflict.
func (c *Client) EnqueueSCMDelivery(ctx context.Context, payload entity.SCMDeliveryPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode source control delivery payload: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeSCMDelivery, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueSCM),
		asynq.MaxRetry(c.maxRetry),
		asynq.TaskID(fmt.Sprintf("scm-delivery:%s:%d", payload.DeliveryID, payload.Attempt)),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue source control delivery: %w", err)
	}

	return nil
}

func (c *Client) EnqueueSCMReconcile(ctx context.Context) error {
	task := asynq.NewTask(entity.TaskTypeSCMReconcile, nil)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueSCM),
		asynq.MaxRetry(c.maxRetry),
		asynq.TaskID(entity.SCMReconcileTaskID),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue source control reconcile: %w", err)
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

func (c *Client) EnqueueSCMResume(ctx context.Context, payload entity.SCMResumePayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode source control resume: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeSCMResume, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueSCM),
		asynq.MaxRetry(c.maxRetry),
		asynq.TaskID(entity.TaskTypeSCMResume+":"+payload.IssueID.String()),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue source control resume: %w", err)
	}

	return nil
}

func (c *Client) EnqueueSCMBackfill(
	ctx context.Context,
	payload entity.SCMBackfillPayload,
) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode source control backfill: %w", err)
	}

	task := asynq.NewTask(entity.TaskTypeSCMBackfill, encoded)

	if _, err := c.producer.EnqueueContext(ctx, task,
		asynq.Queue(entity.QueueSCM),
		asynq.MaxRetry(c.maxRetry),
		asynq.TaskID(entity.TaskTypeSCMBackfill+":"+payload.RepositoryID.String()),
	); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}

		return fmt.Errorf("enqueue source control backfill: %w", err)
	}

	return nil
}

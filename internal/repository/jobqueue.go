package repository

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=jobqueue.go -destination=jobqueue/mock_jobqueue.go -package=jobqueue -mock_names=JobProducer=MockJobProducer,JobInspector=MockJobInspector

type JobProducer interface {
	EnqueueSignUpVerification(ctx context.Context, payload entity.SignUpVerificationPayload) error
	EnqueueEmailChangeConfirmation(ctx context.Context, payload entity.EmailChangeConfirmationPayload) error
	EnqueuePasswordReset(ctx context.Context, payload entity.PasswordResetPayload) error
	EnqueuePasswordResetSSONotice(ctx context.Context, payload entity.PasswordResetSSONoticePayload) error
	EnqueueInvitation(ctx context.Context, payload entity.InvitationPayload) error
	EnqueueWorkspacePurge(ctx context.Context, payload entity.WorkspacePurgePayload, processAt time.Time) error
	EnqueueAttachmentReclaim(ctx context.Context) error
	EnqueueIssuePurge(ctx context.Context, payload entity.IssuePurgePayload, processAt time.Time) error
	EnqueueBulkApply(ctx context.Context, payload entity.BulkApplyPayload) error
	EnqueueWebhookFanOut(ctx context.Context) error
	EnqueueWebhookDeliver(ctx context.Context, payload entity.WebhookDeliverPayload, processAt time.Time) error
	EnqueueImportStage(ctx context.Context, payload entity.ImportStagePayload, processAt time.Time) error
	EnqueueImportExecute(ctx context.Context, payload entity.ImportExecutePayload, processAt time.Time) error
	EnqueueImportRevert(ctx context.Context, payload entity.ImportRevertPayload, processAt time.Time) error
	EnqueueImportRescue(ctx context.Context) error
}

type JobInspector interface {
	Queues(ctx context.Context) ([]entity.QueueStat, error)
	List(ctx context.Context, queue string, state entity.TaskState) ([]entity.TaskSummary, error)
	Run(ctx context.Context, queue, taskID string) error
	Archive(ctx context.Context, queue, taskID string) error
	Delete(ctx context.Context, queue, taskID string) error
}

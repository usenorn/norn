package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=jobqueue.go -destination=jobqueue/mock_jobqueue.go -package=jobqueue -mock_names=JobProducer=MockJobProducer,JobInspector=MockJobInspector

type JobProducer interface {
	EnqueueEmailChangeConfirmation(ctx context.Context, payload entity.EmailChangeConfirmationPayload) error
	EnqueuePasswordReset(ctx context.Context, payload entity.PasswordResetPayload) error
	EnqueuePasswordResetSSONotice(ctx context.Context, payload entity.PasswordResetSSONoticePayload) error
}

type JobInspector interface {
	Queues(ctx context.Context) ([]entity.QueueStat, error)
	List(ctx context.Context, queue string, state entity.TaskState) ([]entity.TaskSummary, error)
	Run(ctx context.Context, queue, taskID string) error
	Archive(ctx context.Context, queue, taskID string) error
	Delete(ctx context.Context, queue, taskID string) error
}

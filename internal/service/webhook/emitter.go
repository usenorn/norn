package webhook

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type emitter struct {
	outbox repository.WebhookOutbox
	jobs   repository.JobProducer
}

func NewEmitter(outbox repository.WebhookOutbox, jobs repository.JobProducer) service.WebhookEmitter {
	return &emitter{outbox: outbox, jobs: jobs}
}

func (e *emitter) Emit(ctx context.Context, entry entity.WebhookOutboxEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}

	if err := e.outbox.Record(ctx, entry); err != nil {
		return err
	}

	postgres.AfterCommit(ctx, func(ctx context.Context) {
		if err := e.jobs.EnqueueWebhookFanOut(ctx); err != nil {
			logging.From(ctx).WarnContext(
				ctx,
				"waking webhook fan-out failed, so the scheduled sweep will carry this event",
				"error", err.Error(),
			)
		}
	})

	return nil
}

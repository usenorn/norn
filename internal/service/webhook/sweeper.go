package webhook

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type sweeper struct {
	retention repository.WebhookRetention
	cfg       config.Webhooks
}

func NewSweeper(retention repository.WebhookRetention, cfg config.Webhooks) service.WebhookRetention {
	return &sweeper{retention: retention, cfg: cfg}
}

func (s *sweeper) Sweep(ctx context.Context) (int, error) {
	cutoff := time.Now().UTC().Add(-s.cfg.Retention)

	deliveries, err := s.drain(ctx, cutoff, s.retention.DropDeliveriesBefore)
	if err != nil {
		return deliveries, err
	}

	events, err := s.drain(ctx, cutoff, s.retention.DropOutboxBefore)
	if err != nil {
		return deliveries + events, err
	}

	dropped := deliveries + events

	if dropped > 0 {
		logging.From(ctx).InfoContext(
			ctx, "webhook history aged out", "deliveries", deliveries, "events", events,
		)
	}

	return dropped, nil
}

func (s *sweeper) drain(
	ctx context.Context,
	cutoff time.Time,
	drop func(context.Context, time.Time, int) (int, error),
) (int, error) {
	dropped := 0

	for {
		batch, err := drop(ctx, cutoff, s.cfg.SweepBatch)
		if err != nil {
			return dropped, err
		}

		dropped += batch

		if batch < s.cfg.SweepBatch {
			return dropped, nil
		}
	}
}

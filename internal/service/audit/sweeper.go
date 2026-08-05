package audit

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type sweeper struct {
	events repository.AuditRetention
	cfg    config.Audit
}

func NewSweeper(events repository.AuditRetention, cfg config.Audit) service.AuditRetention {
	return &sweeper{events: events, cfg: cfg}
}

func (s *sweeper) Sweep(ctx context.Context) (int, error) {
	cutoff := time.Now().UTC().Add(-s.cfg.Retention)
	swept := 0

	for {
		dropped, err := s.events.DropBefore(ctx, cutoff, s.cfg.SweepBatch)
		if err != nil {
			return swept, err
		}

		swept += dropped

		if dropped < s.cfg.SweepBatch {
			break
		}
	}

	if swept > 0 {
		logging.From(ctx).InfoContext(
			ctx,
			"audit retention sweep removed expired records",
			"removed", swept,
			"cutoff", cutoff.Format(time.RFC3339),
		)
	}

	return swept, nil
}

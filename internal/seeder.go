package internal

import (
	"context"
	"log/slog"

	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

type Seeder struct {
	authorizer service.Authorizer
	logger     *slog.Logger
}

func NewSeeder(authorizer service.Authorizer, logger *slog.Logger) *Seeder {
	return &Seeder{authorizer: authorizer, logger: logger}
}

func (s *Seeder) Run(ctx context.Context) error {
	ctx = logging.Into(ctx, s.logger)

	if err := s.authorizer.SeedPolicy(ctx); err != nil {
		return err
	}

	logging.From(ctx).InfoContext(ctx, "authorization policy seeded")

	return nil
}

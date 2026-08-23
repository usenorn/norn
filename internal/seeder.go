package internal

import (
	"context"
	"log/slog"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

type Seeder struct {
	authorizer service.Authorizer
	gateways   service.PreviewGateways
	gateway    config.Gateway
	logger     *slog.Logger
}

func NewSeeder(
	authorizer service.Authorizer,
	gateways service.PreviewGateways,
	gateway config.Gateway,
	logger *slog.Logger,
) *Seeder {
	return &Seeder{
		authorizer: authorizer,
		gateways:   gateways,
		gateway:    gateway,
		logger:     logger,
	}
}

func (s *Seeder) Run(ctx context.Context) error {
	ctx = logging.Into(ctx, s.logger)

	if err := s.authorizer.SeedPolicy(ctx); err != nil {
		return err
	}

	logging.From(ctx).InfoContext(ctx, "authorization policy seeded")

	return s.adoptGateway(ctx)
}

func (s *Seeder) adoptGateway(ctx context.Context) error {
	if s.gateway.Secret == "" {
		return nil
	}

	adopted, err := s.gateways.Adopt(
		ctx, entity.PreviewGatewayConfiguredName, s.gateway.Secret,
	)
	if err != nil {
		return err
	}

	logging.From(ctx).InfoContext(
		ctx,
		"preview gateway credential seeded",
		slog.String("gateway_id", adopted.ID.String()),
		slog.String("name", adopted.Name),
	)

	return nil
}

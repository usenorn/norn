package internal

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"

	"github.com/usenorn/norn/db"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

type Migrator struct {
	provider *goose.Provider
	logger   *slog.Logger
}

func NewMigrator(client *postgres.Client, logger *slog.Logger) (*Migrator, error) {
	fsys, err := db.PostgresMigrations()
	if err != nil {
		return nil, err
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, client.DB, fsys)
	if err != nil {
		return nil, fmt.Errorf("create goose provider: %w", err)
	}

	return &Migrator{provider: provider, logger: logger}, nil
}

func (m *Migrator) Run(ctx context.Context) error {
	ctx = logging.Into(ctx, m.logger)

	results, err := m.provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	for _, result := range results {
		logging.From(ctx).InfoContext(ctx, "migration applied",
			slog.Int64("migration_version", result.Source.Version),
			slog.String("source", result.Source.Path),
			slog.Duration("duration", result.Duration),
		)
	}

	logging.From(ctx).InfoContext(ctx, "migrations up to date", slog.Int("applied", len(results)))

	return nil
}

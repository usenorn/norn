package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

type App struct {
	cfg       config.HTTP
	router    http.Handler
	licensing service.Licensing
	logger    *slog.Logger
}

func NewApp(
	cfg config.HTTP,
	router http.Handler,
	licensing service.Licensing,
	logger *slog.Logger,
) *App {
	return &App{cfg: cfg, router: router, licensing: licensing, logger: logger}
}

func (a *App) Run(ctx context.Context) error {
	ctx = logging.Into(ctx, a.logger)

	a.announceLicence(ctx)

	server := &http.Server{
		Addr:              a.cfg.Addr,
		Handler:           a.router,
		ReadHeaderTimeout: a.cfg.ReadHeaderTimeout,
		ReadTimeout:       a.cfg.ReadTimeout,
		WriteTimeout:      a.cfg.WriteTimeout,
		IdleTimeout:       a.cfg.IdleTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	serving := make(chan error, 1)

	go func() {
		logging.From(ctx).InfoContext(ctx, "http server listening", slog.String("addr", a.cfg.Addr))

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serving <- fmt.Errorf("serve http: %w", err)

			return
		}

		serving <- nil
	}()

	select {
	case err := <-serving:
		return err
	case <-ctx.Done():
	}

	logging.From(ctx).InfoContext(ctx, "http server draining")

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("drain http server: %w", err)
	}

	return <-serving
}

func (a *App) announceLicence(ctx context.Context) {
	report := a.licensing.Report()

	if report.Status == entity.LicenceAbsent {
		logging.From(ctx).InfoContext(
			ctx,
			"no licence key is configured, which is the normal state",
			slog.String("licence_status", string(report.Status)),
		)

		return
	}

	enabled := make([]string, 0, len(report.Features))

	for _, feature := range report.Features {
		if feature.Enabled {
			enabled = append(enabled, string(feature.Name))
		}
	}

	logging.From(ctx).InfoContext(
		ctx,
		"licence loaded",
		slog.String("licence_status", string(report.Status)),
		slog.String("licence_holder", report.Holder),
		slog.Time("licence_expires_at", report.ExpiresAt),
		slog.Time("licence_grace_ends_at", report.GraceEndsAt),
		slog.Any("licence_features", enabled),
	)
}

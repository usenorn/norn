package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/observability/logging"
)

type App struct {
	cfg    config.HTTP
	router http.Handler
	logger *slog.Logger
}

func NewApp(cfg config.HTTP, router http.Handler, logger *slog.Logger) *App {
	return &App{cfg: cfg, router: router, logger: logger}
}

func (a *App) Run(ctx context.Context) error {
	ctx = logging.Into(ctx, a.logger)

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

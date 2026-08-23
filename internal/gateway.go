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
	"github.com/usenorn/norn/internal/service"
)

type Gateway struct {
	cfg      config.Gateway
	previews config.Previews
	router   http.Handler
	proxies  service.PreviewProxy
	logger   *slog.Logger
}

func NewGateway(
	cfg config.Gateway,
	previews config.Previews,
	router http.Handler,
	proxies service.PreviewProxy,
	logger *slog.Logger,
) *Gateway {
	return &Gateway{
		cfg:      cfg,
		previews: previews,
		router:   router,
		proxies:  proxies,
		logger:   logger,
	}
}

func (g *Gateway) Run(ctx context.Context) error {
	ctx = logging.Into(ctx, g.logger)

	if err := g.announce(ctx); err != nil {
		return err
	}

	go g.proxies.Run(ctx)

	server := &http.Server{
		Addr:              g.cfg.Listen,
		Handler:           g.router,
		ReadHeaderTimeout: g.cfg.ReadHeaderTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	serving := make(chan error, 1)

	go func() {
		logging.From(ctx).InfoContext(
			ctx, "preview gateway listening", slog.String("addr", g.cfg.Listen),
		)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serving <- fmt.Errorf("serve preview gateway: %w", err)

			return
		}

		serving <- nil
	}()

	select {
	case err := <-serving:
		return err
	case <-ctx.Done():
	}

	logging.From(ctx).InfoContext(ctx, "preview gateway draining")

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), g.cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("drain preview gateway: %w", err)
	}

	return <-serving
}

func (g *Gateway) announce(ctx context.Context) error {
	if !g.previews.Routable() {
		return errors.New(
			"previews.base_domain is unset, so this gateway has no domain to serve and every " +
				"address would be refused",
		)
	}

	if g.cfg.Server == "" {
		return errors.New(
			"gateway.server is unset, so this gateway has no norn to ask who a viewer is",
		)
	}

	if g.cfg.Secret == "" {
		return errors.New(
			"gateway.secret is unset; mint one with norn preview-gateway enrol <name>",
		)
	}

	logging.From(ctx).InfoContext(
		ctx,
		"preview gateway starting",
		slog.String("preview_domain", g.previews.BaseDomain),
		slog.String("tunnel_host", g.cfg.TunnelAddress(g.previews)),
		slog.String("norn", g.cfg.Server),
	)

	return nil
}

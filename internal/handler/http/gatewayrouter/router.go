package gatewayrouter

import (
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/handler/http/preview"
	"github.com/usenorn/norn/internal/handler/http/previewtunnel"
)

func New(
	cfg config.Gateway,
	httpCfg config.HTTP,
	previews config.Previews,
	previewEdge *preview.Edge,
	tunnelEdge *previewtunnel.Edge,
) http.Handler {
	base := chi.NewRouter()
	base.Use(
		middleware.Recovery,
		middleware.CorrelationID,
		middleware.AccessLog,
		middleware.ClientCapture(httpCfg),
	)

	tunnelHost := bare(cfg.TunnelAddress(previews))

	base.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		if tunnelHost != "" && sameHost(r.Host, tunnelHost) && r.URL.Path == previewtunnel.Path {
			tunnelEdge.Serve(w, r)

			return
		}

		previewEdge.Serve(w, r)
	})

	return base
}

func sameHost(carried, host string) bool {
	return host != "" && bare(carried) == host
}

func bare(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))

	if stripped, _, err := net.SplitHostPort(host); err == nil {
		return stripped
	}

	return strings.Trim(host, "[]")
}

package middleware

import (
	"net/http"
	"strconv"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
)

const mcpThrottleKeyPrefix = "mcp-conn:"

func MCPRateLimit(throttle repository.MCPThrottle, cfg config.MCP) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			connection, ok := MCPConnectionFrom(r.Context())
			if !ok {
				WriteProblem(w, r, http.StatusUnauthorized, "a valid mcp access token is required")

				return
			}

			taken, err := throttle.Record(r.Context(), mcpThrottleKeyPrefix+connection.ID.String())
			if err != nil {
				logging.From(r.Context()).WarnContext(
					r.Context(), "mcp rate limit unavailable", "error", err.Error(),
				)
				WriteProblem(w, r, http.StatusServiceUnavailable, "try again shortly")

				return
			}

			if taken > cfg.RequestsPerWindow {
				w.Header().Set("Retry-After", strconv.Itoa(int(cfg.RateWindow.Seconds())))
				WriteProblem(w, r, http.StatusTooManyRequests, "this connection is sending too many requests")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"net/http"
	"strconv"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
)

const (
	mcpThrottleKeyPrefix = "mcp-token:"
	mcpRunnerKeyPrefix   = "mcp-runner:"
)

func MCPRateLimit(throttle repository.MCPThrottle, cfg config.MCP) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := identity.Actor(r.Context())
			key, keyed := throttleKey(actor)

			if !ok || !keyed {
				rejectToken(w, r)

				return
			}

			taken, err := throttle.Record(r.Context(), key)
			if err != nil {
				logging.From(r.Context()).WarnContext(
					r.Context(), "mcp rate limit unavailable", "error", err.Error(),
				)
				WriteProblem(w, r, http.StatusServiceUnavailable, "try again shortly")

				return
			}

			if taken > cfg.RequestsPerWindow {
				w.Header().Set("Retry-After", strconv.Itoa(int(cfg.RateWindow.Seconds())))
				WriteProblem(w, r, http.StatusTooManyRequests, "this token is sending too many requests")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func throttleKey(actor entity.Actor) (string, bool) {
	switch {
	case actor.TokenID != nil:
		return mcpThrottleKeyPrefix + actor.TokenID.String(), true
	case actor.RunnerID != nil:
		return mcpRunnerKeyPrefix + actor.RunnerID.String(), true
	default:
		return "", false
	}
}

package middleware

import (
	"net/http"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/identity"
)

func MCPEnabled(cfg config.MCP) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				WriteProblem(w, r, http.StatusNotFound, "mcp is not enabled on this instance")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := identity.Actor(r.Context()); !ok {
			rejectToken(w, r)

			return
		}

		next.ServeHTTP(w, r)
	})
}

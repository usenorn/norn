package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

type mcpConnectionKey struct{}

func MCPBearer(
	connections service.MCPConnections,
	app config.App,
	cfg config.MCP,
) func(http.Handler) http.Handler {
	metadata := strings.TrimRight(app.BaseURL, "/") + "/.well-known/oauth-protected-resource/mcp"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				WriteProblem(w, r, http.StatusNotFound, "mcp is not enabled on this instance")

				return
			}

			header := r.Header.Get("Authorization")

			if !strings.HasPrefix(header, bearerPrefix) {
				rejectMCP(w, r, metadata, "invalid_request")

				return
			}

			value := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))

			if !entity.LooksLikeMCPToken(value) {
				rejectMCP(w, r, metadata, "invalid_token")

				return
			}

			actor, connection, err := connections.Authenticate(r.Context(), value)
			if err != nil {
				logging.From(r.Context()).InfoContext(
					r.Context(), "mcp token rejected", "error", err.Error(),
				)
				rejectMCP(w, r, metadata, "invalid_token")

				return
			}

			ctx := identity.WithActor(r.Context(), actor)
			ctx = context.WithValue(ctx, mcpConnectionKey{}, connection)
			ctx = logging.With(
				ctx,
				"account_id", actor.AccountID.String(),
				"connection_id", connection.ID.String(),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func MCPConnectionFrom(ctx context.Context) (entity.MCPConnection, bool) {
	connection, ok := ctx.Value(mcpConnectionKey{}).(entity.MCPConnection)

	return connection, ok
}

func rejectMCP(w http.ResponseWriter, r *http.Request, metadata, code string) {
	w.Header().Set(
		"WWW-Authenticate",
		`Bearer resource_metadata="`+metadata+`", error="`+code+`"`,
	)
	WriteProblem(w, r, http.StatusUnauthorized, "a valid mcp access token is required")
}

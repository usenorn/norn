package middleware

import (
	"net/http"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

const bearerPrefix = "Bearer "

func BearerToken(tokens service.APITokens) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")

			if !strings.HasPrefix(header, bearerPrefix) {
				next.ServeHTTP(w, r)

				return
			}

			value := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))

			if !entity.LooksLikeAPIToken(value) {
				next.ServeHTTP(w, r)

				return
			}

			actor, err := tokens.Authenticate(r.Context(), value)
			if err != nil {
				logging.From(r.Context()).InfoContext(r.Context(), "api token rejected", "error", err.Error())
				next.ServeHTTP(w, r)

				return
			}

			ctx := identity.WithActor(r.Context(), actor)
			ctx = logging.With(
				ctx,
				"account_id", actor.AccountID.String(),
				"token_id", actor.TokenID.String(),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

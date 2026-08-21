package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

const bearerPrefix = "Bearer "

func BearerToken(tokens service.APITokens, runners service.Runners) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")

			if !strings.HasPrefix(header, bearerPrefix) {
				next.ServeHTTP(w, r)

				return
			}

			value := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))

			actor, err := bearerActor(r.Context(), tokens, runners, value)
			if err != nil {
				logging.From(r.Context()).InfoContext(r.Context(), "bearer token rejected", "error", err.Error())
				rejectToken(w, r)

				return
			}

			ctx := identity.WithActor(r.Context(), actor)
			ctx = logging.With(ctx, bearerFields(actor)...)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerActor(
	ctx context.Context,
	tokens service.APITokens,
	runners service.Runners,
	value string,
) (entity.Actor, error) {
	switch {
	case entity.LooksLikeAPIToken(value):
		return tokens.Authenticate(ctx, value)
	case entity.LooksLikeRunnerToken(value):
		return runners.Authenticate(ctx, value)
	default:
		return entity.Actor{}, entity.ErrAPITokenNotFound
	}
}

func bearerFields(actor entity.Actor) []any {
	fields := []any{"account_id", actor.AccountID.String()}

	if actor.TokenID != nil {
		fields = append(fields, "token_id", actor.TokenID.String())
	}

	if actor.RunnerID != nil {
		fields = append(fields, "runner_id", actor.RunnerID.String())
	}

	return fields
}

func rejectToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	WriteProblem(w, r, http.StatusUnauthorized, "that token is not valid")
}

package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/observability/logging"
)

const CorrelationHeader = "X-Correlation-ID"

type correlationKey struct{}

func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get(CorrelationHeader)
		if correlationID == "" {
			correlationID = uuid.NewString()
		}

		w.Header().Set(CorrelationHeader, correlationID)

		ctx := context.WithValue(r.Context(), correlationKey{}, correlationID)
		ctx = logging.With(ctx, "correlation_id", correlationID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CorrelationIDFrom(ctx context.Context) (string, bool) {
	correlationID, ok := ctx.Value(correlationKey{}).(string)

	return correlationID, ok
}

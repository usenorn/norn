package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/usenorn/norn/internal/observability/logging"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			logging.From(r.Context()).ErrorContext(r.Context(), "request panicked",
				"panic", fmt.Sprint(recovered),
				"stack", string(debug.Stack()),
			)

			WriteProblem(w, r, http.StatusInternalServerError, "")
		}()

		next.ServeHTTP(w, r)
	})
}

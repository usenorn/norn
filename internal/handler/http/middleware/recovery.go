package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/usenorn/norn/internal/observability/logging"
)

type committedWriter struct {
	http.ResponseWriter

	committed bool
}

func (w *committedWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *committedWriter) WriteHeader(status int) {
	w.committed = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *committedWriter) Write(payload []byte) (int, error) {
	w.committed = true

	return w.ResponseWriter.Write(payload)
}

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracked := &committedWriter{ResponseWriter: w}

		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(recovered)
			}

			logging.From(r.Context()).ErrorContext(r.Context(), "request panicked",
				"panic", fmt.Sprint(recovered),
				"stack", string(debug.Stack()),
			)

			if tracked.committed {
				return
			}

			WriteProblem(tracked, r, http.StatusInternalServerError, "")
		}()

		next.ServeHTTP(tracked, r)
	})
}

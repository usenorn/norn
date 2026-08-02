package middleware

import (
	"net/http"
	"time"

	"github.com/usenorn/norn/internal/observability/logging"
)

type recordingWriter struct {
	http.ResponseWriter

	status int
	bytes  int
}

func (w *recordingWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *recordingWriter) Write(payload []byte) (int, error) {
	written, err := w.ResponseWriter.Write(payload)
	w.bytes += written

	return written, err
}

func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &recordingWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		logging.From(r.Context()).InfoContext(r.Context(), "request handled",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"bytes", recorder.bytes,
			"duration_ms", time.Since(started).Milliseconds(),
			"remote_ip", r.RemoteAddr,
		)
	})
}

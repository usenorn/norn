package middleware

import (
	"net/http"

	"github.com/usenorn/norn/internal/pkg/httpcookie"
)

// Headers cannot be added once a response has begun, and the generated dashboard responses set
// Set-Cookie themselves with a single value. Handlers therefore hand their cookies to the jar and
// this writer flushes them at the last moment a header can still be written.
func Cookies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, jar := httpcookie.Into(r.Context())

		next.ServeHTTP(&cookieWriter{ResponseWriter: w, jar: jar}, r.WithContext(ctx))
	})
}

type cookieWriter struct {
	http.ResponseWriter
	jar     *httpcookie.Jar
	flushed bool
}

func (w *cookieWriter) WriteHeader(status int) {
	w.flush()
	w.ResponseWriter.WriteHeader(status)
}

func (w *cookieWriter) Write(body []byte) (int, error) {
	w.flush()

	return w.ResponseWriter.Write(body)
}

func (w *cookieWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *cookieWriter) flush() {
	if w.flushed {
		return
	}

	w.flushed = true

	for _, cookie := range w.jar.Cookies() {
		http.SetCookie(w.ResponseWriter, cookie)
	}
}

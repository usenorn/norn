package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

const gzipEncoding = "gzip"

type decompressed struct {
	io.Reader
	body   io.ReadCloser
	packed io.Closer
}

func (d decompressed) Close() error {
	_ = d.packed.Close()

	return d.body.Close()
}

// Decompress unpacks a gzipped request body so a caller sending a large batch pays for it once on
// the wire rather than twice. The unpacked stream carries the same cap as the packed one, because
// a few compressed kilobytes can expand to gigabytes and the limit is the only thing that stops it.
func Decompress(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil || !strings.EqualFold(
				strings.TrimSpace(r.Header.Get("Content-Encoding")), gzipEncoding,
			) {
				next.ServeHTTP(w, r)

				return
			}

			packed, err := gzip.NewReader(r.Body)
			if err != nil {
				WriteProblem(w, r, http.StatusBadRequest, "the body is not the gzip it says it is")

				return
			}

			r.Body = decompressed{
				Reader: http.MaxBytesReader(w, io.NopCloser(packed), limit),
				body:   r.Body,
				packed: packed,
			}

			r.Header.Del("Content-Encoding")
			r.Header.Del("Content-Length")
			r.ContentLength = -1

			next.ServeHTTP(w, r)
		})
	}
}

package middleware_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/handler/http/middleware"
)

const unpackedLimit = 1 << 10

func packed(t *testing.T, payload string) *bytes.Buffer {
	t.Helper()

	var body bytes.Buffer

	writer := gzip.NewWriter(&body)

	if _, err := writer.Write([]byte(payload)); err != nil {
		t.Fatalf("compress a body: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("compress a body: %v", err)
	}

	return &body
}

func post(t *testing.T, body io.Reader, encoding string) (int, string) {
	t.Helper()

	var read string

	handler := middleware.Decompress(unpackedLimit)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusRequestEntityTooLarge)

				return
			}

			read = string(payload)
		},
	))

	request := httptest.NewRequest(http.MethodPost, "/v1/anything", body)
	if encoding != "" {
		request.Header.Set("Content-Encoding", encoding)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder.Code, read
}

func TestAGzippedBodyReachesTheHandlerAsWhatItSaid(t *testing.T) {
	status, read := post(t, packed(t, `{"sequence":1}`), "gzip")

	if status != http.StatusOK || read != `{"sequence":1}` {
		t.Fatalf("the handler saw %q with status %d", read, status)
	}
}

func TestABodyThatSaysNothingAboutEncodingIsPassedThroughUntouched(t *testing.T) {
	status, read := post(t, strings.NewReader(`{"sequence":1}`), "")

	if status != http.StatusOK || read != `{"sequence":1}` {
		t.Fatalf("the handler saw %q with status %d", read, status)
	}
}

func TestABodyThatIsNotTheGzipItClaimsIsRefused(t *testing.T) {
	status, _ := post(t, strings.NewReader("not gzip at all"), "gzip")

	if status != http.StatusBadRequest {
		t.Fatalf("a body that is not gzip answered %d", status)
	}
}

func TestAFewCompressedBytesCannotExpandPastTheLimit(t *testing.T) {
	status, read := post(t, packed(t, strings.Repeat("a", 64<<10)), "gzip")

	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"a body that unpacks to 64KiB against a 1KiB cap answered %d with %d bytes read; "+
				"the packed size is the only thing the outer cap sees, so nothing else stops it",
			status, len(read),
		)
	}
}

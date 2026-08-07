package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/middleware"
)

const ourOrigin = "https://app.norn.so"

func attempt(t *testing.T, method, cookie, origin, referer string) int {
	t.Helper()

	reached := false

	handler := middleware.SameOrigin(
		config.App{BaseURL: ourOrigin},
		config.Session{CookieName: "norn_session"},
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(method, ourOrigin+"/v1/workspaces", nil)

	if cookie != "" {
		request.AddCookie(&http.Cookie{Name: "norn_session", Value: cookie})
	}

	if origin != "" {
		request.Header.Set("Origin", origin)
	}

	if referer != "" {
		request.Header.Set("Referer", referer)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if reached != (recorder.Code != http.StatusForbidden) {
		t.Fatal("the refusal did not stop the request reaching the handler")
	}

	return recorder.Code
}

func TestOnlyOurOwnPagesMaySpendASessionCookie(t *testing.T) {
	cases := map[string]struct {
		method  string
		cookie  string
		origin  string
		referer string
		allowed bool
	}{
		"our own page": {
			method: http.MethodPost, cookie: "live", origin: ourOrigin, allowed: true,
		},
		"another site": {
			method: http.MethodPost, cookie: "live", origin: "https://evil.example.com",
		},
		"a sibling host on our own domain": {
			method: http.MethodPost, cookie: "live", origin: "https://blog.norn.so",
		},
		"our own host over plain http": {
			method: http.MethodPost, cookie: "live", origin: "http://app.norn.so",
		},
		"no origin and no referer": {
			method: http.MethodPost, cookie: "live",
		},
		"an opaque origin": {
			method: http.MethodPost, cookie: "live", origin: "null",
		},
		"falling back to the referer": {
			method: http.MethodPost, cookie: "live", referer: ourOrigin + "/northwind/issues", allowed: true,
		},
		"a token-authenticated caller with no cookie": {
			method: http.MethodPost, allowed: true,
		},
		"reading needs no origin": {
			method: http.MethodGet, cookie: "live", allowed: true,
		},
		"deleting is checked too": {
			method: http.MethodDelete, cookie: "live", origin: "https://evil.example.com",
		},
	}

	for name, tc := range cases {
		status := attempt(t, tc.method, tc.cookie, tc.origin, tc.referer)

		if tc.allowed && status == http.StatusForbidden {
			t.Errorf("%s: refused, but nothing about it is cross-site", name)
		}

		if !tc.allowed && status != http.StatusForbidden {
			t.Errorf(
				"%s: answered %d. A state-changing request carrying the session cookie has to "+
					"prove it came from a page on this site.",
				name, status,
			)
		}
	}
}

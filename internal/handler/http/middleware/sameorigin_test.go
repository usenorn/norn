package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/middleware"
)

const ourOrigin = "https://app.norn.so"

func attempt(t *testing.T, method string, cookies []*http.Cookie, origin, referer string) int {
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

	for _, cookie := range cookies {
		request.AddCookie(cookie)
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

func session(slot string) *http.Cookie {
	return &http.Cookie{Name: "norn_session_" + slot, Value: "live"}
}

func TestOnlyOurOwnPagesMaySpendASessionCookie(t *testing.T) {
	one := []*http.Cookie{session("a1b2c3")}

	cases := map[string]struct {
		method  string
		cookies []*http.Cookie
		origin  string
		referer string
		allowed bool
	}{
		"our own page": {
			method: http.MethodPost, cookies: one, origin: ourOrigin, allowed: true,
		},
		"another site": {
			method: http.MethodPost, cookies: one, origin: "https://evil.example.com",
		},
		"a sibling host on our own domain": {
			method: http.MethodPost, cookies: one, origin: "https://blog.norn.so",
		},
		"our own host over plain http": {
			method: http.MethodPost, cookies: one, origin: "http://app.norn.so",
		},
		"no origin and no referer": {
			method: http.MethodPost, cookies: one,
		},
		"an opaque origin": {
			method: http.MethodPost, cookies: one, origin: "null",
		},
		"falling back to the referer": {
			method: http.MethodPost, cookies: one, referer: ourOrigin + "/northwind/issues", allowed: true,
		},
		"a token-authenticated caller with no cookie": {
			method: http.MethodPost, allowed: true,
		},
		"reading needs no origin": {
			method: http.MethodGet, cookies: one, allowed: true,
		},
		"deleting is checked too": {
			method: http.MethodDelete, cookies: one, origin: "https://evil.example.com",
		},
		"a second account's cookie is checked just the same": {
			method:  http.MethodPost,
			cookies: []*http.Cookie{session("a1b2c3"), session("d4e5f6")},
			origin:  "https://evil.example.com",
		},
		"a cookie that only looks like a session does not trigger the check": {
			method:  http.MethodPost,
			cookies: []*http.Cookie{{Name: "norn_sso", Value: "live"}},
			origin:  "https://evil.example.com",
			allowed: true,
		},
	}

	for name, tc := range cases {
		status := attempt(t, tc.method, tc.cookies, tc.origin, tc.referer)

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

package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/observability/logging"
)

const crossSiteRefusal = "this request did not come from a page on this site"

func SameOrigin(app config.App, session config.Session) func(http.Handler) http.Handler {
	expected := originOf(app.BaseURL)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if readOnly(r.Method) || !carriesSession(r, session.CookieName) {
				next.ServeHTTP(w, r)

				return
			}

			claimed := claimedOrigin(r)
			if claimed == "" || claimed != expected {
				logging.From(r.Context()).WarnContext(r.Context(), "cross-site request refused",
					"claimed_origin", claimed,
					"path", loggedPath(r.URL.Path),
				)
				WriteProblem(w, r, http.StatusForbidden, crossSiteRefusal)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func readOnly(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func carriesSession(r *http.Request, name string) bool {
	cookie, err := r.Cookie(name)

	return err == nil && cookie.Value != ""
}

func claimedOrigin(r *http.Request) string {
	if claimed := r.Header.Get("Origin"); claimed != "" && claimed != "null" {
		return originOf(claimed)
	}

	return originOf(r.Header.Get("Referer"))
}

func originOf(address string) string {
	parsed, err := url.Parse(strings.TrimSpace(address))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}

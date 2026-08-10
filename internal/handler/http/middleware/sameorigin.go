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
			if readOnly(r.Method) || !carriesSession(r, session) {
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

// Deliberately a prefix match rather than a parse: over-matching only means checking the origin
// of a request that did not need it, while under-matching would silently switch the check off.
func carriesSession(r *http.Request, cfg config.Session) bool {
	prefix := sessionCookiePrefix(cfg)

	for _, cookie := range r.Cookies() {
		if strings.HasPrefix(cookie.Name, prefix) && cookie.Value != "" {
			return true
		}
	}

	return false
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

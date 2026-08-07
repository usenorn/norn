package middleware

import (
	"net/http"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
)

const ssoCorrelatorCookieName = "norn_sso"

func SSOCorrelatorCookie(
	cfg config.Session,
	protocol entity.SSOProtocol,
	correlator string,
) *http.Cookie {
	cookie := &http.Cookie{
		Name:     ssoCorrelatorCookieName,
		Value:    correlator,
		Path:     "/",
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	if protocol == entity.SSOProtocolSAML {
		cookie.SameSite = http.SameSiteNoneMode
		cookie.Secure = true
	}

	if correlator == "" {
		cookie.MaxAge = -1
	}

	return cookie
}

func SSOCorrelatorFrom(r *http.Request) string {
	cookie, err := r.Cookie(ssoCorrelatorCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

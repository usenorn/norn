package middleware

import (
	"errors"
	"net/http"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func Session(sessions service.Sessions, cfg config.Session) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cfg.CookieName)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)

				return
			}

			session, err := sessions.Validate(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, entity.ErrSessionNotFound) || errors.Is(err, entity.ErrSessionRevoked) {
					http.SetCookie(w, ExpiredSessionCookie(cfg))
					next.ServeHTTP(w, r)

					return
				}

				logging.From(r.Context()).ErrorContext(r.Context(), "validating session failed", "error", err.Error())
				WriteProblem(w, r, http.StatusInternalServerError, "")

				return
			}

			ctx := identity.WithSession(r.Context(), session)
			ctx = logging.With(ctx, "account_id", session.AccountID.String(), "session_id", session.ID.String())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SessionCookie(cfg config.Session, token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     cfg.CookieName,
		Value:    token,
		Path:     cfg.CookiePath,
		Domain:   cfg.Domain,
		MaxAge:   maxAge,
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: sameSite(cfg.SameSite),
	}
}

func IssuedSessionCookie(cfg config.Session, token string) *http.Cookie {
	return SessionCookie(cfg, token, int(cfg.AbsoluteLifetime.Seconds()))
}

func ExpiredSessionCookie(cfg config.Session) *http.Cookie {
	return SessionCookie(cfg, "", -1)
}

func sameSite(name string) http.SameSite {
	switch name {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

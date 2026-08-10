package middleware

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/httpcookie"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

const (
	sessionSelectorHeader = "X-Norn-Session"
	sessionSelectorQuery  = "s"
	hostCookiePrefix      = "__Host-"
	workspacePathSegment  = "/v1/workspaces/"
)

func Session(sessions service.Sessions, cfg config.Session) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := identity.Actor(r.Context()); ok {
				next.ServeHTTP(w, r)

				return
			}

			presented := presentedSessions(r, cfg)
			if len(presented) == 0 {
				next.ServeHTTP(w, r)

				return
			}

			resolved, err := sessions.Resolve(r.Context(), service.ResolveSessionsInput{
				Presented:   presented,
				Selector:    sessionSelector(r),
				WorkspaceID: workspaceInPath(r.URL.Path),
			})
			if err != nil {
				logging.From(r.Context()).ErrorContext(
					r.Context(), "resolving sessions failed", "error", err.Error(),
				)
				WriteProblem(w, r, http.StatusInternalServerError, "")

				return
			}

			for _, slot := range resolved.Dead {
				httpcookie.Pending(r.Context()).Add(ExpiredSessionCookie(cfg, slot))
			}

			if len(resolved.Held) == 0 {
				next.ServeHTTP(w, r)

				return
			}

			ctx := identity.WithSignedIn(r.Context(), resolved.Held)

			if resolved.Found {
				ctx = identity.WithSession(ctx, resolved.Acting)
				ctx = logging.With(
					ctx,
					"account_id", resolved.Acting.AccountID.String(),
					"session_id", resolved.Acting.ID.String(),
				)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SessionCookieName(cfg config.Session, slot string) string {
	name := cfg.CookieName + "_" + slot
	if cfg.Secure {
		return hostCookiePrefix + name
	}

	return name
}

func SessionCookie(cfg config.Session, slot, token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName(cfg, slot),
		Value:    token,
		Path:     cfg.CookiePath,
		Domain:   cfg.Domain,
		MaxAge:   maxAge,
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: sameSite(cfg.SameSite),
	}
}

func IssuedSessionCookie(cfg config.Session, session entity.Session, token string) *http.Cookie {
	return SessionCookie(cfg, session.Slot, token, int(cfg.AbsoluteLifetime.Seconds()))
}

func ExpiredSessionCookie(cfg config.Session, slot string) *http.Cookie {
	return SessionCookie(cfg, slot, "", -1)
}

func sessionCookiePrefix(cfg config.Session) string {
	if cfg.Secure {
		return hostCookiePrefix + cfg.CookieName + "_"
	}

	return cfg.CookieName + "_"
}

func presentedSessions(r *http.Request, cfg config.Session) []service.PresentedSession {
	prefix := sessionCookiePrefix(cfg)
	presented := make([]service.PresentedSession, 0, 2)

	for _, cookie := range r.Cookies() {
		slot, carried := strings.CutPrefix(cookie.Name, prefix)
		if !carried || slot == "" || cookie.Value == "" {
			continue
		}

		presented = append(presented, service.PresentedSession{Slot: slot, Token: cookie.Value})
	}

	return presented
}

func sessionSelector(r *http.Request) string {
	if selector := r.Header.Get(sessionSelectorHeader); selector != "" {
		return selector
	}

	return r.URL.Query().Get(sessionSelectorQuery)
}

// The one caller that cannot name a session is a workspace-scoped GET whose address was written
// into an issue body long before the browser held several: an attachment renders as an image.
func workspaceInPath(path string) uuid.UUID {
	rest, found := strings.CutPrefix(path, workspacePathSegment)
	if !found {
		return uuid.Nil
	}

	segment, _, _ := strings.Cut(rest, "/")

	workspaceID, err := uuid.Parse(segment)
	if err != nil {
		return uuid.Nil
	}

	return workspaceID
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

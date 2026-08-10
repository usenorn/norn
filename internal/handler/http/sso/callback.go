package sso

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

const (
	CallbackPath = "/v1/sso/oidc/callback"

	exchangeScreen = "/sso"
	settingsScreen = "/settings/authentication"
	overviewScreen = "/settings"
)

var oauthErrorCode = regexp.MustCompile(`^[a-z_]{1,64}$`)

type Callback struct {
	connections service.SSOConnections
	session     config.Session
}

func NewCallback(connections service.SSOConnections, session config.Session) *Callback {
	return &Callback{connections: connections, session: session}
}

func (c *Callback) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	if refusal := query.Get("error"); refusal != "" {
		logging.From(ctx).Warn("provider refused the sign-in",
			"error", refusal,
			"error_description", query.Get("error_description"),
		)

		c.fail(w, r, entity.NewSSOError(
			entity.SSOStageAuthorization,
			providerRefusal(refusal),
		), "")

		return
	}

	exchange, err := c.connections.Complete(ctx, service.CompleteOIDCInput{
		State:      query.Get("state"),
		Code:       query.Get("code"),
		Correlator: middleware.SSOCorrelatorFrom(r),
		Client:     middleware.ClientFrom(ctx),
	})

	http.SetCookie(w, middleware.SSOCorrelatorCookie(c.session, entity.SSOProtocolOIDC, ""))

	if err != nil {
		c.fail(w, r, err, exchange.WorkspaceSlug)

		return
	}

	workspace := "/" + exchange.WorkspaceSlug

	if exchange.Purpose == entity.SSOPurposeTest {
		redirect(w, r, workspace+settingsScreen+"?tested=1")

		return
	}

	if exchange.Purpose == entity.SSOPurposeLink {
		redirect(w, r, workspace+overviewScreen+"?linked=1")

		return
	}

	http.SetCookie(w, middleware.IssuedSessionCookie(c.session, exchange.Session, exchange.Token))

	target := exchange.ReturnTo
	if target == "" {
		target = workspace
	}

	redirect(w, r, target)
}

func (c *Callback) fail(w http.ResponseWriter, r *http.Request, err error, workspace string) {
	failure(w, r, err, workspace)
}

func failure(w http.ResponseWriter, r *http.Request, err error, workspace string) {
	logging.From(r.Context()).Warn("single sign-on exchange failed", "error", err.Error())

	target := url.Values{}

	if errors.Is(err, entity.ErrSSOStateNotFound) {
		err = entity.NewSSOError(
			entity.SSOStageReplay,
			"This sign-in has already been used, or it took too long. Start a new one.",
		)
	}

	if failure, ok := entity.AsSSOError(err); ok {
		target.Set("stage", string(failure.Stage))
		target.Set("message", failure.Message)

		if failure.Subject != "" {
			target.Set("subject", failure.Subject)
		}
	} else {
		target.Set("stage", string(entity.SSOStageAuthorization))
		target.Set("message", "This sign-in attempt could not be completed.")
	}

	if reference, ok := middleware.CorrelationIDFrom(r.Context()); ok {
		target.Set("reference", reference)
	}

	if workspace != "" {
		target.Set("workspace", workspace)
	}

	redirect(w, r, exchangeScreen+"?"+target.Encode())
}

func redirect(w http.ResponseWriter, r *http.Request, target string) {
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func providerRefusal(code string) string {
	if !oauthErrorCode.MatchString(code) {
		return "Your provider refused the sign-in."
	}

	return "Your provider refused the sign-in with " + code + "."
}

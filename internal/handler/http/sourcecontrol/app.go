package sourcecontrol

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

const (
	RegisteredPath = "/v1/source-control/github-app/registered"
	ConnectedPath  = "/v1/source-control/github-app/connected"

	sourceControlScreen = "/settings/source-control"
)

type AppEdge struct {
	apps     service.SourceControlApps
	instance config.App
}

func NewAppEdge(apps service.SourceControlApps, instance config.App) *AppEdge {
	return &AppEdge{apps: apps, instance: instance}
}

func (e *AppEdge) callback() string {
	return strings.TrimRight(e.instance.BaseURL, "/") + ConnectedPath
}

func (e *AppEdge) Registered(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if refusal := query.Get("error"); refusal != "" {
		e.fail(w, r, "", entity.ErrSCMAppRefused)

		return
	}

	attempt, err := e.apps.CompleteRegistration(r.Context(), query.Get("code"), query.Get("state"))
	if err != nil {
		e.fail(w, r, attempt.WorkspaceSlug, err)

		return
	}

	e.done(w, r, attempt.WorkspaceSlug, url.Values{"registered": {"1"}})
}

func (e *AppEdge) Connected(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if refusal := query.Get("error"); refusal != "" {
		e.fail(w, r, "", entity.ErrSCMAppRefused)

		return
	}

	choice, err := e.apps.CompleteAuthorization(
		r.Context(),
		query.Get("code"),
		query.Get("state"),
		e.callback(),
	)
	if err != nil {
		e.fail(w, r, choice.WorkspaceSlug, err)

		return
	}

	e.done(w, r, choice.WorkspaceSlug, url.Values{"installations": {choice.Handle}})
}

func (e *AppEdge) done(
	w http.ResponseWriter,
	r *http.Request,
	workspace string,
	query url.Values,
) {
	http.Redirect(w, r, screen(workspace)+"?"+query.Encode(), http.StatusSeeOther)
}

func (e *AppEdge) fail(w http.ResponseWriter, r *http.Request, workspace string, err error) {
	logging.From(r.Context()).WarnContext(
		r.Context(),
		"a source control application exchange failed",
		"error", err.Error(),
	)

	query := url.Values{"failed": {reason(err)}}

	if reference, ok := middleware.CorrelationIDFrom(r.Context()); ok {
		query.Set("reference", reference)
	}

	http.Redirect(w, r, screen(workspace)+"?"+query.Encode(), http.StatusSeeOther)
}

func reason(err error) string {
	switch {
	case errors.Is(err, entity.ErrSCMAppStateNotFound):
		return "expired"
	case errors.Is(err, entity.ErrSCMAppRefused):
		return "refused"
	case errors.Is(err, entity.ErrSCMAppNotFound):
		return "unregistered"
	default:
		return "unavailable"
	}
}

func screen(workspace string) string {
	if workspace == "" {
		return sourceControlScreen
	}

	return "/" + workspace + sourceControlScreen
}

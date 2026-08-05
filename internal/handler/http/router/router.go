package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/blob"
	"github.com/usenorn/norn/internal/handler/http/events"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/handler/http/sso"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const apiBasePath = "/v1"

func New(
	cfg config.HTTP,
	sessionCfg config.Session,
	attachmentCfg config.Attachments,
	sessions service.Sessions,
	tokens service.APITokens,
	dashboard api.StrictServerInterface,
	callback *sso.Callback,
	samlEdge *sso.SAML,
	blobEdge *blob.Edge,
	eventsEdge *events.Edge,
) http.Handler {
	base := chi.NewRouter()
	base.Use(
		middleware.Recovery,
		middleware.CorrelationID,
		middleware.AccessLog,
		middleware.SecurityHeaders,
		middleware.ClientCapture(cfg),
	)

	bounded := base.With(
		chimiddleware.Timeout(cfg.RequestTimeout),
		maxRequestBytes(cfg.MaxRequestBytes),
	)
	bounded.Get(sso.CallbackPath, callback.Handle)
	bounded.Get(sso.MetadataPath, samlEdge.Metadata)
	bounded.Post(sso.ACSPath, samlEdge.Consume)

	transfers := base.With(chimiddleware.Timeout(attachmentCfg.TransferTimeout))
	transfers.Put(blob.UploadPath, blobEdge.Receive)
	transfers.Get(blob.DownloadPath, blobEdge.Serve)

	// No request timeout: this response is meant to stay open. Session authentication is applied
	// here explicitly because it lives in the generated handler's middleware list, not on base.
	streams := base.With(middleware.Session(sessions, sessionCfg))
	streams.Get(events.Path, eventsEdge.Serve)

	strict := api.NewStrictHandlerWithOptions(dashboard, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			middleware.WriteProblem(w, r, http.StatusBadRequest, err.Error())
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			logging.From(r.Context()).ErrorContext(r.Context(), "request failed", "error", err.Error())
			middleware.WriteProblem(w, r, http.StatusInternalServerError, "")
		},
	})

	return api.HandlerWithOptions(strict, api.ChiServerOptions{
		BaseURL:    apiBasePath,
		BaseRouter: base,
		Middlewares: []api.MiddlewareFunc{
			middleware.BearerToken(tokens),
			middleware.Session(sessions, sessionCfg),
			maxRequestBytes(cfg.MaxRequestBytes),
			chimiddleware.Timeout(cfg.RequestTimeout),
		},
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			middleware.WriteProblem(w, r, http.StatusBadRequest, err.Error())
		},
	})
}

func maxRequestBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}

			next.ServeHTTP(w, r)
		})
	}
}

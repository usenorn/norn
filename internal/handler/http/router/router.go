package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const apiBasePath = "/v1"

func New(
	cfg config.HTTP,
	sessionCfg config.Session,
	sessions service.Sessions,
	tokens service.APITokens,
	dashboard api.StrictServerInterface,
) http.Handler {
	base := chi.NewRouter()
	base.Use(
		middleware.Recovery,
		middleware.CorrelationID,
		middleware.AccessLog,
		chimiddleware.Timeout(cfg.RequestTimeout),
		maxRequestBytes(cfg.MaxRequestBytes),
		middleware.ClientCapture(cfg),
	)

	strict := api.NewStrictHandlerWithOptions(dashboard, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			middleware.WriteProblem(w, r, http.StatusBadRequest, err.Error())
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			middleware.WriteProblem(w, r, http.StatusInternalServerError, "")
		},
	})

	return api.HandlerWithOptions(strict, api.ChiServerOptions{
		BaseURL:    apiBasePath,
		BaseRouter: base,
		Middlewares: []api.MiddlewareFunc{
			middleware.BearerToken(tokens),
			middleware.Session(sessions, sessionCfg),
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

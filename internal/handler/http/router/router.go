package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/auditexport"
	"github.com/usenorn/norn/internal/handler/http/blob"
	"github.com/usenorn/norn/internal/handler/http/events"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/handler/http/runnerchannel"
	"github.com/usenorn/norn/internal/handler/http/scim"
	"github.com/usenorn/norn/internal/handler/http/sourcecontrol"
	"github.com/usenorn/norn/internal/handler/http/sso"
	"github.com/usenorn/norn/internal/handler/mcpserver"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const apiBasePath = "/v1"

func New(
	cfg config.HTTP,
	sessionCfg config.Session,
	attachmentCfg config.Attachments,
	appCfg config.App,
	mcpCfg config.MCP,
	sessions service.Sessions,
	tokens service.APITokens,
	runners service.Runners,
	mcpThrottle repository.MCPThrottle,
	dashboard api.StrictServerInterface,
	callback *sso.Callback,
	samlEdge *sso.SAML,
	blobEdge *blob.Edge,
	eventsEdge *events.Edge,
	auditEdge *auditexport.Edge,
	scimEdge *scim.Edge,
	sourceControlEdge *sourcecontrol.Edge,
	sourceControlApps *sourcecontrol.AppEdge,
	mcpEdge *mcpserver.Edge,
	runnerChannel *runnerchannel.Edge,
) http.Handler {
	base := chi.NewRouter()
	base.Use(
		middleware.Recovery,
		middleware.CorrelationID,
		middleware.AccessLog,
		middleware.SecurityHeaders,
		middleware.Cookies,
		middleware.ClientCapture(cfg),
		middleware.SameOrigin(appCfg, sessionCfg),
	)

	bounded := base.With(
		chimiddleware.Timeout(cfg.RequestTimeout),
		maxRequestBytes(cfg.MaxRequestBytes),
	)
	bounded.Get(sso.CallbackPath, callback.Handle)
	bounded.Get(sso.MetadataPath, samlEdge.Metadata)
	bounded.Post(sso.ACSPath, samlEdge.Consume)

	bounded.Get(sourcecontrol.RegisteredPath, sourceControlApps.Registered)
	bounded.Get(sourcecontrol.ConnectedPath, sourceControlApps.Connected)

	// A forge chooses its own payload size and its own timeout, so this route carries its
	// own byte cap rather than the dashboard's, which is sized for a form.
	deliveries := base.With(chimiddleware.Timeout(cfg.RequestTimeout))
	deliveries.Post(sourcecontrol.DeliveryPath, sourceControlEdge.Deliver)
	deliveries.Post(sourcecontrol.AppDeliveryPath, sourceControlEdge.DeliverToApp)

	transfers := base.With(chimiddleware.Timeout(attachmentCfg.TransferTimeout))
	transfers.Put(blob.UploadPath, blobEdge.Receive)
	transfers.Get(blob.DownloadPath, blobEdge.Serve)

	mcpOps := base.With(
		middleware.MCPEnabled(mcpCfg),
		middleware.BearerToken(tokens, runners),
		middleware.RequireToken,
		middleware.MCPRateLimit(mcpThrottle, mcpCfg),
		maxRequestBytes(cfg.MaxRequestBytes),
		chimiddleware.Timeout(cfg.RequestTimeout),
	)
	mcpOps.Handle(mcpserver.Path, mcpEdge)

	streams := base.With(middleware.Session(sessions, sessionCfg))
	streams.Get(events.Path, eventsEdge.Serve)

	base.Get(runnerchannel.Path, runnerChannel.Serve)

	exports := base.With(
		middleware.BearerToken(tokens, runners),
		middleware.Session(sessions, sessionCfg),
	)
	exports.Get(auditexport.WorkspacePath, auditEdge.ServeWorkspace)
	exports.Get(auditexport.InstancePath, auditEdge.ServeInstance)

	directory := base.With(
		chimiddleware.Timeout(cfg.RequestTimeout),
		maxRequestBytes(cfg.MaxRequestBytes),
	)
	directory.Get(scim.ConfigPath, scimEdge.ServiceProviderConfig)
	directory.Get(scim.TypesPath, scimEdge.ResourceTypes)
	directory.Get(scim.SchemasPath, scimEdge.Schemas)
	directory.Get(scim.UsersPath, scimEdge.ListUsers)
	directory.Post(scim.UsersPath, scimEdge.CreateUser)
	directory.Get(scim.UserPath, scimEdge.GetUser)
	directory.Put(scim.UserPath, scimEdge.ReplaceUser)
	directory.Patch(scim.UserPath, scimEdge.PatchUser)
	directory.Delete(scim.UserPath, scimEdge.DeleteUser)
	directory.Get(scim.GroupsPath, scimEdge.ListGroups)
	directory.Post(scim.GroupsPath, scimEdge.CreateGroup)
	directory.Get(scim.GroupPath, scimEdge.GetGroup)
	directory.Put(scim.GroupPath, scimEdge.ReplaceGroup)
	directory.Patch(scim.GroupPath, scimEdge.PatchGroup)
	directory.Delete(scim.GroupPath, scimEdge.DeleteGroup)

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
			maxRequestBytes(cfg.MaxRequestBytes),
			chimiddleware.Timeout(cfg.RequestTimeout),
			middleware.BearerToken(tokens, runners),
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

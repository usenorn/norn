package previewtunnel

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coder/websocket"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const Path = channelv1.TunnelPath

type Edge struct {
	proxies service.PreviewProxy
	cfg     config.Gateway
}

func New(proxies service.PreviewProxy, cfg config.Gateway) *Edge {
	return &Edge{proxies: proxies, cfg: cfg}
}

func (e *Edge) Serve(w http.ResponseWriter, r *http.Request) {
	ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	if ticket == "" {
		middleware.WriteProblem(
			w, r, http.StatusUnauthorized,
			"open the tunnel with the ticket from /v1/runners/token",
		)

		return
	}

	claim, err := e.proxies.Accept(r.Context(), ticket)
	if err != nil {
		e.refuse(w, r, err)

		return
	}

	socket, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}

	ctx := logging.With(
		r.Context(),
		slog.String("runner_id", claim.RunnerID.String()),
		slog.String("workspace_id", claim.WorkspaceID.String()),
	)

	logging.From(ctx).InfoContext(ctx, "a machine opened a preview tunnel")

	carried := websocket.NetConn(ctx, socket, websocket.MessageBinary)

	if err := e.proxies.Hold(ctx, claim.RunnerID, carried); err != nil {
		logging.From(ctx).InfoContext(
			ctx, "a preview tunnel closed", slog.String("reason", err.Error()),
		)
	}

	_ = socket.CloseNow()
}

func (e *Edge) refuse(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, entity.ErrGatewayUnready):
		middleware.WriteProblem(w, r, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, entity.ErrRunnerRevoked), errors.Is(err, entity.ErrAgentDisabled):
		middleware.WriteProblem(w, r, http.StatusForbidden, err.Error())
	case errors.Is(err, entity.ErrPreviewGatewayCredentialInvalid):
		middleware.WriteProblem(w, r, http.StatusServiceUnavailable, err.Error())
	default:
		middleware.WriteProblem(
			w, r, http.StatusUnauthorized, entity.ErrRunnerCredentialInvalid.Error(),
		)
	}
}

package previewgateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

const (
	BasePath       = entity.PreviewGatewayBasePath
	TokenPath      = entity.PreviewGatewayTokenPath
	IntrospectPath = entity.PreviewGatewayIntrospectPath
	SessionPath    = entity.PreviewGatewaySessionPath
	SharePath      = entity.PreviewGatewaySharePath
	TunnelPath     = entity.PreviewGatewayTunnelPath

	bearer = "Bearer "
)

type Edge struct {
	previews service.Previews
	gateways service.PreviewGateways
	runners  service.Runners
}

func New(
	previews service.Previews,
	gateways service.PreviewGateways,
	runners service.Runners,
) *Edge {
	return &Edge{previews: previews, gateways: gateways, runners: runners}
}

type tokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type introspectRequest struct {
	Host      string `json:"host"`
	Grant     string `json:"grant"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
}

type introspectResponse struct {
	Verdict     string     `json:"verdict"`
	ExecutionID string     `json:"executionId,omitempty"`
	RunnerID    string     `json:"runnerId,omitempty"`
	Preview     string     `json:"preview,omitempty"`
	Mode        string     `json:"mode,omitempty"`
	Path        string     `json:"path,omitempty"`
	Reason      string     `json:"reason,omitempty"`
	Redirect    string     `json:"redirect,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

type sessionRequest struct {
	Ticket string `json:"ticket"`
}

type tunnelRequest struct {
	Ticket string `json:"ticket"`
}

type tunnelResponse struct {
	RunnerID    string `json:"runnerId"`
	WorkspaceID string `json:"workspaceId"`
	Runner      string `json:"runner"`
}

type shareRequest struct {
	Host     string `json:"host"`
	Token    string `json:"token"`
	Passcode string `json:"passcode"`
}

type grantResponse struct {
	Grant     string    `json:"grant"`
	ExpiresAt time.Time `json:"expiresAt"`
	Cookie    string    `json:"cookie"`
	Path      string    `json:"path,omitempty"`
}

func (e *Edge) Exchange(w http.ResponseWriter, r *http.Request) {
	secret := presented(r)
	if secret == "" {
		refuse(w, r)

		return
	}

	access, err := e.gateways.Exchange(r.Context(), secret)
	if err != nil {
		if errors.Is(err, entity.ErrPreviewGatewayCredentialInvalid) ||
			errors.Is(err, entity.ErrPreviewGatewayRevoked) {
			refuse(w, r)

			return
		}

		e.fail(w, r, err)

		return
	}

	answer(w, r, http.StatusOK, tokenResponse{
		Token:     access.Token,
		ExpiresAt: access.ExpiresAt,
	})
}

func (e *Edge) Introspect(w http.ResponseWriter, r *http.Request) {
	if _, ok := e.calling(w, r); !ok {
		return
	}

	var asked introspectRequest
	if !read(w, r, &asked) {
		return
	}

	access, err := e.previews.Introspect(
		r.Context(), strings.ToLower(strings.TrimSpace(asked.Host)), asked.Grant, viewer(asked),
	)
	if err != nil {
		e.fail(w, r, err)

		return
	}

	answered := introspectResponse{
		Verdict:     string(access.Verdict),
		ExecutionID: access.Preview.ExecutionID,
		Preview:     access.Preview.Name,
		Mode:        string(access.Preview.Mode),
		Path:        access.Preview.Path,
		Reason:      access.Reason,
		Redirect:    access.Redirect,
	}

	if access.RunnerID != uuid.Nil {
		answered.RunnerID = access.RunnerID.String()
	}

	if !access.ExpiresAt.IsZero() {
		answered.ExpiresAt = &access.ExpiresAt
	}

	answer(w, r, http.StatusOK, answered)
}

func (e *Edge) Session(w http.ResponseWriter, r *http.Request) {
	if _, ok := e.calling(w, r); !ok {
		return
	}

	var asked sessionRequest
	if !read(w, r, &asked) {
		return
	}

	access, err := e.previews.RedeemTicket(r.Context(), asked.Ticket)
	if err != nil {
		if errors.Is(err, entity.ErrPreviewGrantNotFound) {
			middleware.WriteProblem(w, r, http.StatusGone, err.Error())

			return
		}

		e.fail(w, r, err)

		return
	}

	answer(w, r, http.StatusCreated, granted(access))
}

func (e *Edge) Tunnel(w http.ResponseWriter, r *http.Request) {
	if _, ok := e.calling(w, r); !ok {
		return
	}

	var asked tunnelRequest
	if !read(w, r, &asked) {
		return
	}

	runner, err := e.runners.AcceptTunnel(r.Context(), strings.TrimSpace(asked.Ticket))
	if err != nil {
		switch {
		case errors.Is(err, entity.ErrRunnerCredentialInvalid),
			errors.Is(err, entity.ErrRunnerNotFound):
			middleware.WriteProblem(
				w, r, http.StatusUnauthorized, entity.ErrRunnerCredentialInvalid.Error(),
			)
		case errors.Is(err, entity.ErrRunnerRevoked), errors.Is(err, entity.ErrAgentDisabled):
			middleware.WriteProblem(w, r, http.StatusForbidden, err.Error())
		default:
			e.fail(w, r, err)
		}

		return
	}

	answer(w, r, http.StatusCreated, tunnelResponse{
		RunnerID:    runner.ID.String(),
		WorkspaceID: runner.WorkspaceID.String(),
		Runner:      runner.Name,
	})
}

func (e *Edge) Share(w http.ResponseWriter, r *http.Request) {
	if _, ok := e.calling(w, r); !ok {
		return
	}

	var asked shareRequest
	if !read(w, r, &asked) {
		return
	}

	access, err := e.previews.Redeem(
		r.Context(),
		strings.ToLower(strings.TrimSpace(asked.Host)),
		asked.Token,
		asked.Passcode,
	)
	if err != nil {
		e.refused(w, r, err)

		return
	}

	answer(w, r, http.StatusCreated, granted(access))
}

func (e *Edge) calling(w http.ResponseWriter, r *http.Request) (entity.PreviewGateway, bool) {
	gateway, err := e.gateways.Authenticate(r.Context(), presented(r))
	if err != nil {
		refuse(w, r)

		return entity.PreviewGateway{}, false
	}

	return gateway, true
}

func (e *Edge) refused(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, entity.ErrPreviewNotFound),
		errors.Is(err, entity.ErrPreviewShareNotFound):
		middleware.WriteProblem(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, entity.ErrPreviewClosed),
		errors.Is(err, entity.ErrPreviewShareExpired),
		errors.Is(err, entity.ErrPreviewShareRevoked):
		middleware.WriteProblem(w, r, http.StatusGone, err.Error())
	case errors.Is(err, entity.ErrPreviewSharePasscodeNeeded):
		middleware.WriteProblem(w, r, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, entity.ErrPreviewSharePasscode):
		middleware.WriteProblem(w, r, http.StatusForbidden, err.Error())
	case errors.Is(err, entity.ErrPreviewShareGuessed):
		middleware.WriteProblem(w, r, http.StatusTooManyRequests, err.Error())
	default:
		e.fail(w, r, err)
	}
}

func (e *Edge) fail(w http.ResponseWriter, r *http.Request, err error) {
	logging.From(r.Context()).ErrorContext(
		r.Context(), "a preview gateway request failed", "error", err.Error(),
	)

	middleware.WriteProblem(w, r, http.StatusInternalServerError, "")
}

func granted(access entity.PreviewAccess) grantResponse {
	return grantResponse{
		Grant:     access.Token,
		ExpiresAt: access.ExpiresAt,
		Cookie:    entity.PreviewGrantCookie,
		Path:      access.Path,
	}
}

func viewer(asked introspectRequest) entity.SessionClient {
	client := entity.SessionClient{
		UserAgent: entity.TruncateUserAgent(asked.UserAgent),
	}

	if address, err := netip.ParseAddr(strings.TrimSpace(asked.IP)); err == nil {
		client.IP = address
	}

	return client
}

func presented(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, bearer) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(header, bearer))
}

func read(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		middleware.WriteProblem(w, r, http.StatusBadRequest, "that request body is not valid JSON")

		return false
	}

	return true
}

func answer(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		logging.From(r.Context()).ErrorContext(
			r.Context(), "a preview gateway answer could not be written", "error", err.Error(),
		)
	}
}

func refuse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	middleware.WriteProblem(
		w, r, http.StatusUnauthorized, entity.ErrPreviewGatewayCredentialInvalid.Error(),
	)
}

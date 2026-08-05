package mcpauth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const (
	ResourceMetadataPath    = "/.well-known/oauth-protected-resource"
	ResourceMetadataMCPPath = "/.well-known/oauth-protected-resource/mcp"
	AuthServerMetadataPath  = "/.well-known/oauth-authorization-server"
	RegisterPath            = "/oauth/register"
	AuthorizePath           = "/oauth/authorize"
	TokenPath               = "/oauth/token"
	RevokePath              = "/oauth/revoke"

	consentPath  = "/authorize"
	mcpPath      = "/mcp"
	dcrKeyPrefix = "mcp-dcr:"
)

type Edge struct {
	connections service.MCPConnections
	throttle    repository.MCPThrottle
	app         config.App
	cfg         config.MCP
}

func New(
	connections service.MCPConnections,
	throttle repository.MCPThrottle,
	app config.App,
	cfg config.MCP,
) *Edge {
	return &Edge{connections: connections, throttle: throttle, app: app, cfg: cfg}
}

func (e *Edge) enabled(w http.ResponseWriter, r *http.Request) bool {
	if e.cfg.Enabled {
		return true
	}

	middleware.WriteProblem(w, r, http.StatusNotFound, "mcp is not enabled on this instance")

	return false
}

func (e *Edge) base() string {
	return strings.TrimRight(e.app.BaseURL, "/")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)

	return decoder.Decode(target)
}

type oauthErrorBody struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, oauthErrorBody{Error: code, Description: description})
}

func redirectWith(redirectURI string, params map[string]string) (string, bool) {
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return "", false
	}

	query := parsed.Query()

	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}

	parsed.RawQuery = query.Encode()

	return parsed.String(), true
}

func oauthErrorCode(err error) string {
	switch {
	case errors.Is(err, entity.ErrMCPRequestInvalid):
		return "invalid_request"
	case errors.Is(err, entity.ErrMCPScopeInvalid):
		return "invalid_scope"
	case errors.Is(err, entity.ErrMCPResourceInvalid):
		return "invalid_target"
	default:
		return ""
	}
}

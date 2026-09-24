package mcpoauth

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
	CallbackPath = "/v1/agent-mcp/oauth/callback"

	outcomeParam    = "connection"
	referenceParam  = "reference"
	connected       = "connected"
	entryScreen     = "/"
	fragmentDivider = "#"
	queryDivider    = "?"
	paramJoiner     = "&"
)

type Callback struct {
	capabilities service.AgentCapabilities
	app          config.App
}

func NewCallback(capabilities service.AgentCapabilities, app config.App) *Callback {
	return &Callback{capabilities: capabilities, app: app}
}

func (c *Callback) redirectURI() string {
	return strings.TrimRight(c.app.BaseURL, "/") + CallbackPath
}

func (c *Callback) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	code := query.Get("code")
	if refusal := query.Get("error"); refusal != "" {
		logging.From(ctx).WarnContext(ctx, "an mcp authorization server refused the sign-in", "error", refusal)

		code = ""
	}

	completed, err := c.capabilities.CompleteMCPConnect(ctx, query.Get("state"), code, c.redirectURI())
	if err != nil {
		logging.From(ctx).WarnContext(ctx, "an mcp sign-in could not be completed", "error", err.Error())

		outcome := url.Values{outcomeParam: {failure(err)}}
		if reference, ok := middleware.CorrelationIDFrom(ctx); ok {
			outcome.Set(referenceParam, reference)
		}

		http.Redirect(w, r, returnTo(completed.ReturnTo, outcome), http.StatusSeeOther)

		return
	}

	http.Redirect(w, r, returnTo(completed.ReturnTo, url.Values{outcomeParam: {connected}}), http.StatusSeeOther)
}

func failure(err error) string {
	switch {
	case errors.Is(err, entity.ErrAgentMCPOAuthStateNotFound):
		return "expired"
	case errors.Is(err, entity.ErrAgentMCPOAuthRefused):
		return "refused"
	case errors.Is(err, entity.ErrAgentMCPServerNotFound):
		return "removed"
	default:
		return "failed"
	}
}

func returnTo(target string, outcome url.Values) string {
	if !entity.ValidAgentMCPReturnTo(target) {
		target = entryScreen
	}

	target, _, _ = strings.Cut(target, fragmentDivider)

	divider := queryDivider
	if strings.Contains(target, queryDivider) {
		divider = paramJoiner
	}

	return target + divider + outcome.Encode()
}

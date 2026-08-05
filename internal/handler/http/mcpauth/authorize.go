package mcpauth

import (
	"errors"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/service"
)

func (e *Edge) Authorize(w http.ResponseWriter, r *http.Request) {
	if !e.enabled(w, r) {
		return
	}

	query := r.URL.Query()

	input := service.BeginMCPAuthorizationInput{
		ClientID:            query.Get("client_id"),
		RedirectURI:         query.Get("redirect_uri"),
		ResponseType:        query.Get("response_type"),
		Scope:               query.Get("scope"),
		State:               query.Get("state"),
		CodeChallenge:       query.Get("code_challenge"),
		CodeChallengeMethod: query.Get("code_challenge_method"),
		Resource:            query.Get("resource"),
	}

	requestID, err := e.connections.BeginAuthorization(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, entity.ErrMCPClientNotFound):
			middleware.WriteProblem(w, r, http.StatusBadRequest, "the client is not registered")
		case errors.Is(err, entity.ErrMCPRedirectInvalid):
			middleware.WriteProblem(
				w, r, http.StatusBadRequest, "the redirect uri is not registered for this client",
			)
		default:
			if code := oauthErrorCode(err); code != "" {
				if target, ok := redirectWith(input.RedirectURI, map[string]string{
					"error": code,
					"state": input.State,
				}); ok {
					http.Redirect(w, r, target, http.StatusFound)

					return
				}
			}

			middleware.WriteProblem(w, r, http.StatusInternalServerError, "")
		}

		return
	}

	http.Redirect(w, r, e.base()+consentPath+"?request="+requestID, http.StatusFound)
}

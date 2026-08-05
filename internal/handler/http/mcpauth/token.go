package mcpauth

import (
	"errors"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (e *Edge) Token(w http.ResponseWriter, r *http.Request) {
	if !e.enabled(w, r) {
		return
	}

	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "the request body is not a form")

		return
	}

	var (
		pair service.MCPTokenPair
		err  error
	)

	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		pair, err = e.connections.Exchange(r.Context(), service.ExchangeMCPCodeInput{
			ClientID:     r.PostFormValue("client_id"),
			Code:         r.PostFormValue("code"),
			RedirectURI:  r.PostFormValue("redirect_uri"),
			CodeVerifier: r.PostFormValue("code_verifier"),
		})
	case "refresh_token":
		pair, err = e.connections.Refresh(r.Context(), service.RefreshMCPTokenInput{
			ClientID:     r.PostFormValue("client_id"),
			RefreshToken: r.PostFormValue("refresh_token"),
		})
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "")

		return
	}

	if err != nil {
		switch {
		case errors.Is(err, entity.ErrMCPCodeInvalid):
			writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
		case errors.Is(err, entity.ErrMCPClientNotFound):
			writeOAuthError(w, http.StatusBadRequest, "invalid_client", "")
		default:
			writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		}

		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  pair.AccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    pair.ExpiresIn,
		RefreshToken: pair.RefreshToken,
		Scope:        string(pair.Capability),
	})
}

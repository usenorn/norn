package mcpauth

import (
	"net/http"
)

func (e *Edge) Revoke(w http.ResponseWriter, r *http.Request) {
	if !e.enabled(w, r) {
		return
	}

	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "the request body is not a form")

		return
	}

	if err := e.connections.RevokeByValue(
		r.Context(),
		r.PostFormValue("token"),
		r.PostFormValue("client_id"),
	); err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")

		return
	}

	w.WriteHeader(http.StatusOK)
}

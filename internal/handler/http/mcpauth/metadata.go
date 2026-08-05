package mcpauth

import "net/http"

func (e *Edge) ProtectedResource(w http.ResponseWriter, r *http.Request) {
	if !e.enabled(w, r) {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 e.base() + mcpPath,
		"authorization_servers":    []string{e.base()},
		"scopes_supported":         []string{"read", "write"},
		"bearer_methods_supported": []string{"header"},
	})
}

func (e *Edge) AuthorizationServer(w http.ResponseWriter, r *http.Request) {
	if !e.enabled(w, r) {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                e.base(),
		"authorization_endpoint":                e.base() + AuthorizePath,
		"token_endpoint":                        e.base() + TokenPath,
		"registration_endpoint":                 e.base() + RegisterPath,
		"revocation_endpoint":                   e.base() + RevokePath,
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                      []string{"read", "write"},
	})
}

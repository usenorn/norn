package mcpauth

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/service"
)

type registrationRequest struct {
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type registrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
}

func (e *Edge) Register(w http.ResponseWriter, r *http.Request) {
	if !e.enabled(w, r) {
		return
	}

	client := middleware.ClientFrom(r.Context())

	taken, err := e.throttle.Record(r.Context(), dcrKeyPrefix+client.IP.String())
	if err != nil {
		w.Header().Set("Retry-After", strconv.Itoa(int(e.cfg.RateWindow.Seconds())))
		writeOAuthError(
			w, http.StatusServiceUnavailable,
			"temporarily_unavailable", "registrations cannot be counted right now",
		)

		return
	}

	if taken > entity.MCPRegistrationsPerWindow {
		w.Header().Set("Retry-After", strconv.Itoa(int(e.cfg.RateWindow.Seconds())))
		writeOAuthError(
			w, http.StatusTooManyRequests,
			"invalid_client_metadata", "too many registrations from this address",
		)

		return
	}

	var request registrationRequest

	if err := decodeJSON(r, &request); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "the request body is not valid JSON")

		return
	}

	registered, err := e.connections.RegisterClient(r.Context(), service.RegisterMCPClientInput{
		Name:         request.ClientName,
		RedirectURIs: request.RedirectURIs,
	})
	if err != nil {
		var validation entity.ValidationError
		if errors.As(err, &validation) {
			code := "invalid_client_metadata"

			for _, field := range validation.Fields {
				if field.Field == "redirect_uris" {
					code = "invalid_redirect_uri"
				}
			}

			writeOAuthError(w, http.StatusBadRequest, code, validation.Error())

			return
		}

		writeOAuthError(w, http.StatusInternalServerError, "invalid_client_metadata", "")

		return
	}

	writeJSON(w, http.StatusCreated, registrationResponse{
		ClientID:                registered.ID.String(),
		ClientName:              registered.Name,
		RedirectURIs:            registered.RedirectURIs,
		TokenEndpointAuthMethod: "none",
		ClientIDIssuedAt:        time.Now().UTC().Unix(),
	})
}

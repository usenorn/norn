package sso

import (
	"encoding/base64"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/service"
)

const (
	MetadataPath = "/v1/sso/saml/{workspace}/metadata"
	ACSPath      = "/v1/sso/saml/{workspace}/acs"

	metadataContentType = "application/samlmetadata+xml"
	responseField       = "SAMLResponse"
	relayStateField     = "RelayState"
)

type SAML struct {
	connections service.SSOConnections
	session     config.Session
}

func NewSAML(connections service.SSOConnections, session config.Session) *SAML {
	return &SAML{connections: connections, session: session}
}

func (s *SAML) Metadata(w http.ResponseWriter, r *http.Request) {
	document, err := s.connections.PublishSAMLMetadata(r.Context(), chi.URLParam(r, "workspace"))
	if err != nil {
		middleware.WriteProblem(w, r, http.StatusNotFound, err.Error())

		return
	}

	w.Header().Set("Content-Type", metadataContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(document)
}

func (s *SAML) Consume(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspace := chi.URLParam(r, "workspace")

	if err := r.ParseForm(); err != nil {
		failure(w, r, entity.SSOFailure(
			entity.SSOStageResponse,
			"The provider's response could not be read.",
			err,
		), workspace)

		return
	}

	encoded := r.PostFormValue(responseField)
	if encoded == "" {
		failure(w, r, entity.NewSSOError(
			entity.SSOStageResponse,
			"The provider sent nothing Norn could read as a SAML response.",
		), workspace)

		return
	}

	document, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		failure(w, r, entity.SSOFailure(
			entity.SSOStageResponse,
			"The provider's response was not valid base64.",
			err,
		), workspace)

		return
	}

	exchange, err := s.connections.CompleteSAML(ctx, service.CompleteSAMLInput{
		WorkspaceSlug: workspace,
		RelayState:    r.PostFormValue(relayStateField),
		Response:      document,
		Client:        middleware.ClientFrom(ctx),
	})
	if err != nil {
		failure(w, r, err, workspace)

		return
	}

	target := "/" + exchange.WorkspaceSlug

	if exchange.Purpose == entity.SSOPurposeTest {
		redirect(w, r, target+settingsScreen+"?tested=1")

		return
	}

	http.SetCookie(w, middleware.IssuedSessionCookie(s.session, exchange.Token))

	if exchange.ReturnTo != "" {
		target = exchange.ReturnTo
	}

	redirect(w, r, target)
}

package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) GetWorkspaceSsoProtocol(
	ctx context.Context,
	request api.GetWorkspaceSsoProtocolRequestObject,
) (api.GetWorkspaceSsoProtocolResponseObject, error) {
	protocol, err := h.ssoConnections.Protocol(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceSsoProtocol200JSONResponse{
		WorkspaceId: request.WorkspaceId,
		Protocol:    api.SsoProtocol(protocol),
	}, nil
}

func (h *handler) RemoveWorkspaceSsoConnection(
	ctx context.Context,
	request api.RemoveWorkspaceSsoConnectionRequestObject,
) (api.RemoveWorkspaceSsoConnectionResponseObject, error) {
	if err := h.ssoConnections.Remove(ctx, request.WorkspaceId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceSsoConnection204Response{}, nil
}

func (h *handler) GetWorkspaceSamlConnection(
	ctx context.Context,
	request api.GetWorkspaceSamlConnectionRequestObject,
) (api.GetWorkspaceSamlConnectionResponseObject, error) {
	connection, err := h.ssoConnections.GetSAML(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceSamlConnection200JSONResponse(h.samlConnectionDTO(ctx, connection)), nil
}

func (h *handler) SetWorkspaceSamlConnection(
	ctx context.Context,
	request api.SetWorkspaceSamlConnectionRequestObject,
) (api.SetWorkspaceSamlConnectionResponseObject, error) {
	input := service.SaveSAMLConnectionInput{WorkspaceID: request.WorkspaceId}

	if request.Body.MetadataUrl != nil {
		input.MetadataURL = *request.Body.MetadataUrl
	}

	if request.Body.Metadata != nil {
		input.Metadata = *request.Body.Metadata
	}

	if request.Body.Descriptor != nil {
		descriptor := samlDescriptor(*request.Body.Descriptor)
		input.Descriptor = &descriptor
	}

	if request.Body.AllowIdpInitiated != nil {
		input.AllowIDPInitiated = *request.Body.AllowIdpInitiated
	}

	if request.Body.Provisioning != nil {
		input.Provisioning = *request.Body.Provisioning
	}

	if request.Body.Mapping != nil {
		input.Mapping = samlMapping(*request.Body.Mapping)
	}

	if request.Body.AdminGroup != nil {
		input.AdminGroup = *request.Body.AdminGroup
	}

	connection, err := h.ssoConnections.SaveSAML(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceSamlConnection200JSONResponse(h.samlConnectionDTO(ctx, connection)), nil
}

func (h *handler) ReadSamlMetadata(
	ctx context.Context,
	request api.ReadSamlMetadataRequestObject,
) (api.ReadSamlMetadataResponseObject, error) {
	input := service.ReadSAMLMetadataInput{WorkspaceID: request.WorkspaceId}

	if request.Body.MetadataUrl != nil {
		input.MetadataURL = *request.Body.MetadataUrl
	}

	if request.Body.Metadata != nil {
		input.Metadata = *request.Body.Metadata
	}

	descriptor, err := h.ssoConnections.ReadSAMLMetadata(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReadSamlMetadata200JSONResponse(samlDescriptorDTO(descriptor)), nil
}

func (h *handler) TestWorkspaceSamlConnection(
	ctx context.Context,
	request api.TestWorkspaceSamlConnectionRequestObject,
) (api.TestWorkspaceSamlConnectionResponseObject, error) {
	handoff, err := h.ssoConnections.BeginSAMLTest(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	h.fileSSOCorrelator(ctx, entity.SSOProtocolSAML, handoff.Correlator)

	return api.TestWorkspaceSamlConnection200JSONResponse(api.OidcAuthorization{AuthorizationUrl: handoff.AuthorizationURL}), nil
}

func (h *handler) BeginSamlLogin(
	ctx context.Context,
	request api.BeginSamlLoginRequestObject,
) (api.BeginSamlLoginResponseObject, error) {
	input := service.BeginOIDCLoginInput{WorkspaceSlug: request.Body.Workspace}

	if request.Body.ReturnTo != nil {
		input.ReturnTo = *request.Body.ReturnTo
	}

	handoff, err := h.ssoConnections.BeginSAMLLogin(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	h.fileSSOCorrelator(ctx, entity.SSOProtocolSAML, handoff.Correlator)

	return api.BeginSamlLogin200JSONResponse(api.OidcAuthorization{AuthorizationUrl: handoff.AuthorizationURL}), nil
}

func (h *handler) samlConnectionDTO(
	ctx context.Context,
	connection entity.SAMLConnection,
) api.WorkspaceSamlConnection {
	slug := h.workspaceSlug(ctx, connection.WorkspaceID)
	base := h.oidcRedirectBase() + "/v1/sso/saml/" + slug
	daysLeft := int32(entity.DaysUntil(connection.Descriptor.ExpiresAt, time.Now().UTC()))

	dto := api.WorkspaceSamlConnection{
		WorkspaceId:          connection.WorkspaceID,
		Descriptor:           samlDescriptorDTO(connection.Descriptor),
		SpEntityId:           connection.SPEntityID,
		SpCertificate:        connection.SPCertificate,
		AcsUrl:               base + "/acs",
		MetadataUrl:          base + "/metadata",
		AllowIdpInitiated:    connection.AllowIDPInitiated,
		Mapping:              samlMappingDTO(connection.Mapping),
		Provisioning:         connection.Provisioning,
		CertificateExpiresAt: connection.Descriptor.ExpiresAt,
		CertificateDaysLeft:  &daysLeft,
		VerifiedAt:           connection.VerifiedAt,
		UpdatedAt:            connection.UpdatedAt,
	}

	if connection.MetadataURL != "" {
		providerMetadata := connection.MetadataURL
		dto.ProviderMetadataUrl = &providerMetadata
	}

	if connection.AdminGroup != "" {
		group := connection.AdminGroup
		dto.AdminGroup = &group
	}

	signIn := h.oidcRedirectBase() + "/sso?workspace=" + slug
	dto.SignInUrl = &signIn

	return dto
}

func samlDescriptorDTO(descriptor entity.SAMLDescriptor) api.SamlDescriptor {
	dto := api.SamlDescriptor{
		EntityId:     descriptor.EntityID,
		SsoUrl:       descriptor.SSOURL,
		Certificates: descriptor.Certificates,
		ExpiresAt:    descriptor.ExpiresAt,
	}

	if descriptor.SLOURL != "" {
		logout := descriptor.SLOURL
		dto.SloUrl = &logout
	}

	return dto
}

func samlDescriptor(dto api.SamlDescriptor) entity.SAMLDescriptor {
	descriptor := entity.SAMLDescriptor{
		EntityID:     dto.EntityId,
		SSOURL:       dto.SsoUrl,
		Certificates: dto.Certificates,
	}

	if dto.SloUrl != nil {
		descriptor.SLOURL = *dto.SloUrl
	}

	return descriptor
}

func samlMappingDTO(mapping entity.SAMLAttributeMapping) api.SamlAttributeMapping {
	dto := api.SamlAttributeMapping{}

	if mapping.Email != "" {
		email := mapping.Email
		dto.Email = &email
	}

	if mapping.Name != "" {
		name := mapping.Name
		dto.Name = &name
	}

	if mapping.Groups != "" {
		groups := mapping.Groups
		dto.Groups = &groups
	}

	return dto
}

func samlMapping(dto api.SamlAttributeMapping) entity.SAMLAttributeMapping {
	mapping := entity.SAMLAttributeMapping{}

	if dto.Email != nil {
		mapping.Email = *dto.Email
	}

	if dto.Name != nil {
		mapping.Name = *dto.Name
	}

	if dto.Groups != nil {
		mapping.Groups = *dto.Groups
	}

	return mapping
}

func (r problemResponse) VisitGetWorkspaceSsoProtocolResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceSsoConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceSamlConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceSamlConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReadSamlMetadataResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitTestWorkspaceSamlConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitBeginSamlLoginResponse(w http.ResponseWriter) error {
	return r.write(w)
}

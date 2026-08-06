package dashboard

import (
	"context"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const oidcCallbackPath = "/v1/sso/oidc/callback"

func (h *handler) GetWorkspaceOidcConnection(
	ctx context.Context,
	request api.GetWorkspaceOidcConnectionRequestObject,
) (api.GetWorkspaceOidcConnectionResponseObject, error) {
	connection, err := h.ssoConnections.Get(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceOidcConnection200JSONResponse(h.oidcConnectionDTO(connection)), nil
}

func (h *handler) SetWorkspaceOidcConnection(
	ctx context.Context,
	request api.SetWorkspaceOidcConnectionRequestObject,
) (api.SetWorkspaceOidcConnectionResponseObject, error) {
	input := service.SaveOIDCConnectionInput{
		WorkspaceID: request.WorkspaceId,
		Issuer:      request.Body.Issuer,
		ClientID:    request.Body.ClientId,
	}

	if request.Body.Endpoints != nil {
		endpoints := oidcEndpoints(*request.Body.Endpoints)
		input.Endpoints = &endpoints
	}

	if request.Body.ClientSecret != nil {
		input.ClientSecret = *request.Body.ClientSecret
	}

	if request.Body.Scopes != nil {
		input.Scopes = *request.Body.Scopes
	}

	if request.Body.GroupsClaim != nil {
		input.GroupsClaim = *request.Body.GroupsClaim
	}

	if request.Body.Provisioning != nil {
		input.Provisioning = *request.Body.Provisioning
	}

	connection, err := h.ssoConnections.Save(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceOidcConnection200JSONResponse(h.oidcConnectionDTO(connection)), nil
}

func (h *handler) RemoveWorkspaceOidcConnection(
	ctx context.Context,
	request api.RemoveWorkspaceOidcConnectionRequestObject,
) (api.RemoveWorkspaceOidcConnectionResponseObject, error) {
	if err := h.ssoConnections.Remove(ctx, request.WorkspaceId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceOidcConnection204Response{}, nil
}

func (h *handler) DiscoverOidcEndpoints(
	ctx context.Context,
	request api.DiscoverOidcEndpointsRequestObject,
) (api.DiscoverOidcEndpointsResponseObject, error) {
	endpoints, err := h.ssoConnections.Discover(ctx, request.WorkspaceId, request.Body.Issuer)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DiscoverOidcEndpoints200JSONResponse(oidcEndpointsDTO(endpoints)), nil
}

func (h *handler) TestWorkspaceOidcConnection(
	ctx context.Context,
	request api.TestWorkspaceOidcConnectionRequestObject,
) (api.TestWorkspaceOidcConnectionResponseObject, error) {
	target, err := h.ssoConnections.BeginTest(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.TestWorkspaceOidcConnection200JSONResponse{AuthorizationUrl: target}, nil
}

func (h *handler) LinkWorkspaceOidcIdentity(
	ctx context.Context,
	request api.LinkWorkspaceOidcIdentityRequestObject,
) (api.LinkWorkspaceOidcIdentityResponseObject, error) {
	target, err := h.ssoConnections.BeginLink(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.LinkWorkspaceOidcIdentity200JSONResponse{AuthorizationUrl: target}, nil
}

func (h *handler) GetWorkspaceSignIn(
	ctx context.Context,
	request api.GetWorkspaceSignInRequestObject,
) (api.GetWorkspaceSignInResponseObject, error) {
	signIn, err := h.ssoConnections.SignIn(ctx, request.Params.Workspace)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	body := api.GetWorkspaceSignIn200JSONResponse{
		Workspace: signIn.Slug,
		Name:      signIn.Name,
		Password:  signIn.Password,
		Sso:       signIn.SSO,
	}

	if signIn.Protocol != "" {
		protocol := api.SsoProtocol(signIn.Protocol)
		body.Protocol = &protocol
	}

	if signIn.Host != "" {
		host := signIn.Host
		body.Host = &host
	}

	return body, nil
}

func (h *handler) BeginOidcLogin(
	ctx context.Context,
	request api.BeginOidcLoginRequestObject,
) (api.BeginOidcLoginResponseObject, error) {
	input := service.BeginOIDCLoginInput{WorkspaceSlug: request.Body.Workspace}

	if request.Body.ReturnTo != nil {
		input.ReturnTo = *request.Body.ReturnTo
	}

	target, err := h.ssoConnections.BeginLogin(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.BeginOidcLogin200JSONResponse{AuthorizationUrl: target}, nil
}

func (h *handler) oidcConnectionDTO(connection entity.OIDCConnection) api.WorkspaceOidcConnection {
	scopes := connection.Scopes
	if scopes == nil {
		scopes = []string{}
	}

	dto := api.WorkspaceOidcConnection{
		WorkspaceId:  connection.WorkspaceID,
		Endpoints:    oidcEndpointsDTO(connection.Endpoints),
		Discovered:   connection.Discovered,
		ClientId:     connection.ClientID,
		SecretSet:    connection.ClientSecret != "",
		Scopes:       scopes,
		Provisioning: connection.Provisioning,
		RedirectUri:  h.oidcRedirectURI(),
		VerifiedAt:   connection.VerifiedAt,
		UpdatedAt:    connection.UpdatedAt,
	}

	if connection.GroupsClaim != "" {
		claim := connection.GroupsClaim
		dto.GroupsClaim = &claim
	}

	return dto
}

func oidcEndpointsDTO(endpoints entity.OIDCEndpoints) api.OidcEndpoints {
	dto := api.OidcEndpoints{
		Issuer:                endpoints.Issuer,
		AuthorizationEndpoint: endpoints.AuthorizationEndpoint,
		TokenEndpoint:         endpoints.TokenEndpoint,
		JwksUri:               endpoints.JWKSURI,
	}

	if endpoints.UserinfoEndpoint != "" {
		userinfo := endpoints.UserinfoEndpoint
		dto.UserinfoEndpoint = &userinfo
	}

	return dto
}

func oidcEndpoints(dto api.OidcEndpoints) entity.OIDCEndpoints {
	endpoints := entity.OIDCEndpoints{
		Issuer:                dto.Issuer,
		AuthorizationEndpoint: dto.AuthorizationEndpoint,
		TokenEndpoint:         dto.TokenEndpoint,
		JWKSURI:               dto.JwksUri,
	}

	if dto.UserinfoEndpoint != nil {
		endpoints.UserinfoEndpoint = *dto.UserinfoEndpoint
	}

	return endpoints
}

func (r problemResponse) VisitGetWorkspaceOidcConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceOidcConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceOidcConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDiscoverOidcEndpointsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitTestWorkspaceOidcConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitLinkWorkspaceOidcIdentityResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceSignInResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitBeginOidcLoginResponse(w http.ResponseWriter) error {
	return r.write(w)
}

package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListAPITokens(
	ctx context.Context,
	_ api.ListAPITokensRequestObject,
) (api.ListAPITokensResponseObject, error) {
	tokens, err := h.apiTokens.List(ctx)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListAPITokens200JSONResponse(apiTokenDTOs(tokens)), nil
}

func (h *handler) MintAPIToken(
	ctx context.Context,
	request api.MintAPITokenRequestObject,
) (api.MintAPITokenResponseObject, error) {
	minted, err := h.apiTokens.Mint(ctx, service.MintAPITokenInput{
		Name:      request.Body.Name,
		Scopes:    apiScopes(request.Body.Scopes),
		Grants:    apiTokenGrants(request.Body.Grants),
		ExpiresAt: request.Body.ExpiresAt,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.MintAPIToken201JSONResponse{
		Token: apiTokenDTO(minted.Token),
		Value: minted.Value,
	}, nil
}

func apiScopes(scopes []api.APIScope) entity.APIScopeSet {
	values := make(entity.APIScopeSet, 0, len(scopes))

	for _, scope := range scopes {
		values = append(values, entity.APIScope(scope))
	}

	return values
}

func (h *handler) RevokeAPIToken(
	ctx context.Context,
	request api.RevokeAPITokenRequestObject,
) (api.RevokeAPITokenResponseObject, error) {
	if err := h.apiTokens.Revoke(ctx, request.TokenId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevokeAPIToken204Response{}, nil
}

func (h *handler) ListWorkspaceAPITokens(
	ctx context.Context,
	request api.ListWorkspaceAPITokensRequestObject,
) (api.ListWorkspaceAPITokensResponseObject, error) {
	tokens, err := h.apiTokens.ListForWorkspace(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceAPITokens200JSONResponse(workspaceAPITokenDTOs(tokens)), nil
}

func (h *handler) RevokeWorkspaceAPIToken(
	ctx context.Context,
	request api.RevokeWorkspaceAPITokenRequestObject,
) (api.RevokeWorkspaceAPITokenResponseObject, error) {
	if err := h.apiTokens.RevokeInWorkspace(ctx, request.WorkspaceId, request.TokenId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevokeWorkspaceAPIToken204Response{}, nil
}

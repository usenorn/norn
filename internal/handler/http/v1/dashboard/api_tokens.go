package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceAPITokens(
	ctx context.Context,
	request api.ListWorkspaceAPITokensRequestObject,
) (api.ListWorkspaceAPITokensResponseObject, error) {
	tokens, err := h.apiTokens.List(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceAPITokens200JSONResponse(apiTokenDTOs(tokens)), nil
}

func (h *handler) MintWorkspaceAPIToken(
	ctx context.Context,
	request api.MintWorkspaceAPITokenRequestObject,
) (api.MintWorkspaceAPITokenResponseObject, error) {
	input := service.MintAPITokenInput{
		WorkspaceID: request.WorkspaceId,
		Name:        request.Body.Name,
		Scopes:      entity.NewAPIScopeSet(request.Body.Scopes),
		ExpiresAt:   request.Body.ExpiresAt,
	}

	minted, err := h.apiTokens.Mint(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.MintWorkspaceAPIToken201JSONResponse{
		Token: apiTokenDTO(minted.Token),
		Value: minted.Value,
	}, nil
}

func (h *handler) RevokeWorkspaceAPIToken(
	ctx context.Context,
	request api.RevokeWorkspaceAPITokenRequestObject,
) (api.RevokeWorkspaceAPITokenResponseObject, error) {
	if err := h.apiTokens.Revoke(ctx, request.WorkspaceId, request.TokenId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevokeWorkspaceAPIToken204Response{}, nil
}

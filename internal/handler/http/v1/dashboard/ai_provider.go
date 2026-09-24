package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) GetWorkspaceAiProvider(
	ctx context.Context,
	request api.GetWorkspaceAiProviderRequestObject,
) (api.GetWorkspaceAiProviderResponseObject, error) {
	connection, err := h.aiProviders.Get(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceAiProvider200JSONResponse(aiProviderDTO(connection)), nil
}

func (h *handler) SetWorkspaceAiProvider(
	ctx context.Context,
	request api.SetWorkspaceAiProviderRequestObject,
) (api.SetWorkspaceAiProviderResponseObject, error) {
	connection, err := h.aiProviders.Configure(ctx, request.WorkspaceId, entity.AIProviderInput{
		Provider: entity.AIProviderKind(request.Body.Provider),
		Endpoint: entity.AIProviderEndpoint{
			BaseURL:             optionalString(request.Body.BaseUrl),
			AllowPrivateAddress: request.Body.AllowPrivateAddress != nil && *request.Body.AllowPrivateAddress,
		},
		APIKey:       optionalString(request.Body.ApiKey),
		DefaultModel: optionalString(request.Body.DefaultModel),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceAiProvider200JSONResponse(aiProviderDTO(connection)), nil
}

func (h *handler) RemoveWorkspaceAiProvider(
	ctx context.Context,
	request api.RemoveWorkspaceAiProviderRequestObject,
) (api.RemoveWorkspaceAiProviderResponseObject, error) {
	if err := h.aiProviders.Remove(ctx, request.WorkspaceId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceAiProvider204Response{}, nil
}

func (h *handler) ListWorkspaceAiProviderModels(
	ctx context.Context,
	request api.ListWorkspaceAiProviderModelsRequestObject,
) (api.ListWorkspaceAiProviderModelsResponseObject, error) {
	models, err := h.aiProviders.Models(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceAiProviderModels200JSONResponse{Models: models}, nil
}

func (h *handler) TestWorkspaceAiProvider(
	ctx context.Context,
	request api.TestWorkspaceAiProviderRequestObject,
) (api.TestWorkspaceAiProviderResponseObject, error) {
	connection, err := h.aiProviders.Test(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.TestWorkspaceAiProvider200JSONResponse(aiProviderDTO(connection)), nil
}

func aiProviderDTO(connection entity.AIProviderConnection) api.WorkspaceAiProvider {
	dto := api.WorkspaceAiProvider{
		Provider:            api.AiProviderKind(connection.Provider),
		BaseUrl:             connection.Endpoint.BaseURL,
		AllowPrivateAddress: connection.Endpoint.AllowPrivateAddress,
		KeyHint:             connection.KeyHint,
		DefaultModel:        connection.DefaultModel,
		Status:              api.AiProviderStatus(connection.Status()),
		VerifiedAt:          connection.VerifiedAt,
		FailedAt:            connection.FailedAt,
		CreatedAt:           connection.CreatedAt,
		UpdatedAt:           connection.UpdatedAt,
	}

	if connection.Failure != "" {
		failure := api.AiProviderFailure(connection.Failure)
		dto.Failure = &failure
	}

	return dto
}

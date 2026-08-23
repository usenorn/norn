package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ConnectCodebase(
	ctx context.Context,
	request api.ConnectCodebaseRequestObject,
) (api.ConnectCodebaseResponseObject, error) {
	input := service.ConnectCodebaseInput{
		RootPath:     request.Body.RootPath,
		Repositories: codebaseRepositoriesOf(request.Body.Repositories),
		SharedFiles:  sharedFilesOf(request.Body.SharedFiles),
		Runtimes:     codebaseRuntimesOf(request.Body.Runtimes),
		Tools:        codingToolsOf(request.Body.Tools),
	}

	if request.Body.PreviewGateway != nil {
		input.PreviewGateway = entity.GatewayReach(*request.Body.PreviewGateway)
	}

	if request.Body.Name != nil {
		input.Name = *request.Body.Name
	}

	codebase, err := h.codebases.Connect(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConnectCodebase200JSONResponse(codebaseDTO(codebase)), nil
}

func (h *handler) ListCurrentRunnerCodebases(
	ctx context.Context,
	_ api.ListCurrentRunnerCodebasesRequestObject,
) (api.ListCurrentRunnerCodebasesResponseObject, error) {
	codebases, err := h.codebases.Mine(ctx)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListCurrentRunnerCodebases200JSONResponse(codebaseDTOs(codebases)), nil
}

func (h *handler) ConfirmCodebase(
	ctx context.Context,
	request api.ConfirmCodebaseRequestObject,
) (api.ConfirmCodebaseResponseObject, error) {
	codebase, err := h.codebases.Confirm(ctx, request.CodebaseId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConfirmCodebase200JSONResponse(codebaseDTO(codebase)), nil
}

func (h *handler) DisconnectCodebase(
	ctx context.Context,
	request api.DisconnectCodebaseRequestObject,
) (api.DisconnectCodebaseResponseObject, error) {
	codebase, err := h.codebases.Disconnect(ctx, request.CodebaseId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DisconnectCodebase200JSONResponse(codebaseDTO(codebase)), nil
}

func (h *handler) ListAgentCodebases(
	ctx context.Context,
	request api.ListAgentCodebasesRequestObject,
) (api.ListAgentCodebasesResponseObject, error) {
	codebases, err := h.codebases.ListByAgent(ctx, request.WorkspaceId, request.AgentId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListAgentCodebases200JSONResponse(codebaseDTOs(codebases)), nil
}

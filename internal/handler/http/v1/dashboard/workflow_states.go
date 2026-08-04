package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkflowStates(
	ctx context.Context,
	request api.ListWorkflowStatesRequestObject,
) (api.ListWorkflowStatesResponseObject, error) {
	states, err := h.workflowStates.List(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkflowStates200JSONResponse(workflowStateDTOs(states)), nil
}

func (h *handler) CreateWorkflowState(
	ctx context.Context,
	request api.CreateWorkflowStateRequestObject,
) (api.CreateWorkflowStateResponseObject, error) {
	state, err := h.workflowStates.Create(ctx, service.CreateWorkflowStateInput{
		WorkspaceID: request.WorkspaceId,
		TeamID:      request.TeamId,
		Name:        request.Body.Name,
		Category:    entity.StateCategory(request.Body.Category),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkflowState201JSONResponse(workflowStateDTO(state)), nil
}

func (h *handler) UpdateWorkflowState(
	ctx context.Context,
	request api.UpdateWorkflowStateRequestObject,
) (api.UpdateWorkflowStateResponseObject, error) {
	input := service.UpdateWorkflowStateInput{Name: request.Body.Name}

	if request.Body.Category != nil {
		category := entity.StateCategory(*request.Body.Category)
		input.Category = &category
	}

	state, err := h.workflowStates.Update(ctx, request.WorkspaceId, request.TeamId, request.StateId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkflowState200JSONResponse(workflowStateDTO(state)), nil
}

func (h *handler) ReorderWorkflowStates(
	ctx context.Context,
	request api.ReorderWorkflowStatesRequestObject,
) (api.ReorderWorkflowStatesResponseObject, error) {
	states, err := h.workflowStates.Reorder(ctx, request.WorkspaceId, request.TeamId, request.Body.StateIds)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReorderWorkflowStates200JSONResponse(workflowStateDTOs(states)), nil
}

func (h *handler) SetDefaultWorkflowState(
	ctx context.Context,
	request api.SetDefaultWorkflowStateRequestObject,
) (api.SetDefaultWorkflowStateResponseObject, error) {
	states, err := h.workflowStates.SetDefault(ctx, request.WorkspaceId, request.TeamId, request.StateId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetDefaultWorkflowState200JSONResponse(workflowStateDTOs(states)), nil
}

func (h *handler) SetCompletionWorkflowState(
	ctx context.Context,
	request api.SetCompletionWorkflowStateRequestObject,
) (api.SetCompletionWorkflowStateResponseObject, error) {
	states, err := h.workflowStates.SetCompletion(ctx, request.WorkspaceId, request.TeamId, request.StateId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetCompletionWorkflowState200JSONResponse(workflowStateDTOs(states)), nil
}

func (h *handler) RemoveWorkflowState(
	ctx context.Context,
	request api.RemoveWorkflowStateRequestObject,
) (api.RemoveWorkflowStateResponseObject, error) {
	if err := h.workflowStates.Remove(
		ctx,
		request.WorkspaceId,
		request.TeamId,
		request.StateId,
		request.Params.ReplacementStateId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkflowState204Response{}, nil
}

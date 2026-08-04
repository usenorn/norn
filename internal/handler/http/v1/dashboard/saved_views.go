package dashboard

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceSavedViews(
	ctx context.Context,
	request api.ListWorkspaceSavedViewsRequestObject,
) (api.ListWorkspaceSavedViewsResponseObject, error) {
	summaries, err := h.savedViews.List(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceSavedViews200JSONResponse(savedViewDTOs(summaries)), nil
}

func (h *handler) CreateWorkspaceSavedView(
	ctx context.Context,
	request api.CreateWorkspaceSavedViewRequestObject,
) (api.CreateWorkspaceSavedViewResponseObject, error) {
	input := service.CreateSavedViewInput{
		WorkspaceID: request.WorkspaceId,
		Name:        request.Body.Name,
		Filter:      issueFilterFrom(request.Body.Filter),
	}

	if request.Body.Sharing != nil {
		input.Sharing = entity.SavedViewSharing(*request.Body.Sharing)
	}

	if request.Body.TeamId != nil {
		input.TeamID = *request.Body.TeamId
	}

	if request.Body.Sort != nil {
		input.Sort = issueSortFrom(*request.Body.Sort)
	}

	if request.Body.GroupBy != nil {
		input.GroupBy = entity.IssueGroupBy(*request.Body.GroupBy)
	}

	summary, err := h.savedViews.Create(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceSavedView201JSONResponse(savedViewDTO(summary)), nil
}

func (h *handler) GetWorkspaceSavedView(
	ctx context.Context,
	request api.GetWorkspaceSavedViewRequestObject,
) (api.GetWorkspaceSavedViewResponseObject, error) {
	detail, err := h.savedViews.Get(ctx, request.WorkspaceId, request.SavedViewId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceSavedView200JSONResponse(savedViewDetailDTO(detail)), nil
}

func (h *handler) UpdateWorkspaceSavedView(
	ctx context.Context,
	request api.UpdateWorkspaceSavedViewRequestObject,
) (api.UpdateWorkspaceSavedViewResponseObject, error) {
	input := service.UpdateSavedViewInput{
		Name:   request.Body.Name,
		Filter: issueFilterFrom(request.Body.Filter),
	}

	if request.Body.Sharing != nil {
		sharing := entity.SavedViewSharing(*request.Body.Sharing)
		input.Sharing = &sharing
	}

	if request.Body.TeamId != nil {
		team := *request.Body.TeamId
		input.TeamID = &team
	}

	if request.Body.Sort != nil {
		sort := issueSortFrom(*request.Body.Sort)
		input.Sort = &sort
	}

	if request.Body.GroupBy != nil {
		groupBy := entity.IssueGroupBy(*request.Body.GroupBy)
		input.GroupBy = &groupBy
	}

	summary, err := h.savedViews.Update(ctx, request.WorkspaceId, request.SavedViewId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceSavedView200JSONResponse(savedViewDTO(summary)), nil
}

func (h *handler) RemoveWorkspaceSavedView(
	ctx context.Context,
	request api.RemoveWorkspaceSavedViewRequestObject,
) (api.RemoveWorkspaceSavedViewResponseObject, error) {
	err := h.savedViews.Remove(
		ctx, request.WorkspaceId, request.SavedViewId,
		entity.SavedViewSharing(request.Params.AcknowledgedSharing),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceSavedView204Response{}, nil
}

func (h *handler) ReorderWorkspaceSavedViews(
	ctx context.Context,
	request api.ReorderWorkspaceSavedViewsRequestObject,
) (api.ReorderWorkspaceSavedViewsResponseObject, error) {
	orderedIDs := make([]uuid.UUID, 0, len(request.Body.SavedViewIds))
	orderedIDs = append(orderedIDs, request.Body.SavedViewIds...)

	summaries, err := h.savedViews.Reorder(ctx, request.WorkspaceId, orderedIDs)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReorderWorkspaceSavedViews200JSONResponse(savedViewDTOs(summaries)), nil
}

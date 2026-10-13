package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

var publicRoadmapCache = "no-store"

func (h *handler) GetWorkspaceRoadmap(
	ctx context.Context,
	request api.GetWorkspaceRoadmapRequestObject,
) (api.GetWorkspaceRoadmapResponseObject, error) {
	view, err := h.roadmaps.Get(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceRoadmap200JSONResponse(roadmapDTO(view)), nil
}

func (h *handler) UpdateWorkspaceRoadmap(
	ctx context.Context,
	request api.UpdateWorkspaceRoadmapRequestObject,
) (api.UpdateWorkspaceRoadmapResponseObject, error) {
	view, err := h.roadmaps.Update(ctx, request.WorkspaceId, service.UpdateRoadmapInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		Title:           request.Body.Title,
		Description:     request.Body.Description,
		Visibility:      roadmapVisibility(request.Body.Visibility),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceRoadmap200JSONResponse(roadmapDTO(view)), nil
}

func (h *handler) PublishWorkspaceRoadmap(
	ctx context.Context,
	request api.PublishWorkspaceRoadmapRequestObject,
) (api.PublishWorkspaceRoadmapResponseObject, error) {
	view, err := h.roadmaps.Publish(ctx, request.WorkspaceId, service.PublishRoadmapInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.PublishWorkspaceRoadmap200JSONResponse(roadmapDTO(view)), nil
}

func (h *handler) UnpublishWorkspaceRoadmap(
	ctx context.Context,
	request api.UnpublishWorkspaceRoadmapRequestObject,
) (api.UnpublishWorkspaceRoadmapResponseObject, error) {
	view, err := h.roadmaps.Unpublish(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnpublishWorkspaceRoadmap200JSONResponse(roadmapDTO(view)), nil
}

func (h *handler) CreateWorkspaceRoadmapEntry(
	ctx context.Context,
	request api.CreateWorkspaceRoadmapEntryRequestObject,
) (api.CreateWorkspaceRoadmapEntryResponseObject, error) {
	view, err := h.roadmaps.AddEntry(ctx, request.WorkspaceId, service.AddRoadmapEntryInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		Kind:            roadmapEntryKind(request.Body.Kind),
		SourceID:        request.Body.SourceId,
		Content:         roadmapEntryContent(request.Body.Content),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceRoadmapEntry201JSONResponse(roadmapDTO(view)), nil
}

func (h *handler) UpdateWorkspaceRoadmapEntry(
	ctx context.Context,
	request api.UpdateWorkspaceRoadmapEntryRequestObject,
) (api.UpdateWorkspaceRoadmapEntryResponseObject, error) {
	view, err := h.roadmaps.UpdateEntry(ctx, request.WorkspaceId, request.EntryId, service.UpdateRoadmapEntryInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		Content:         roadmapEntryContent(request.Body.Content),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceRoadmapEntry200JSONResponse(roadmapDTO(view)), nil
}

func (h *handler) DeleteWorkspaceRoadmapEntry(
	ctx context.Context,
	request api.DeleteWorkspaceRoadmapEntryRequestObject,
) (api.DeleteWorkspaceRoadmapEntryResponseObject, error) {
	view, err := h.roadmaps.RemoveEntry(ctx, request.WorkspaceId, request.EntryId, int(request.Params.ExpectedVersion))
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteWorkspaceRoadmapEntry200JSONResponse(roadmapDTO(view)), nil
}

func (h *handler) RefreshWorkspaceRoadmapEntry(
	ctx context.Context,
	request api.RefreshWorkspaceRoadmapEntryRequestObject,
) (api.RefreshWorkspaceRoadmapEntryResponseObject, error) {
	view, err := h.roadmaps.RefreshEntry(ctx, request.WorkspaceId, request.EntryId, int(request.Body.ExpectedVersion))
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RefreshWorkspaceRoadmapEntry200JSONResponse(roadmapDTO(view)), nil
}

func (h *handler) GetPublicRoadmap(
	ctx context.Context,
	_ api.GetPublicRoadmapRequestObject,
) (api.GetPublicRoadmapResponseObject, error) {
	roadmap, err := h.publicRoadmaps.Get(ctx, identity.Host(ctx))
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetPublicRoadmap200JSONResponse{
		Body:    publicRoadmapDTO(roadmap),
		Headers: api.GetPublicRoadmap200ResponseHeaders{CacheControl: &publicRoadmapCache},
	}, nil
}

func (h *handler) GetPublicRoadmapEntry(
	ctx context.Context,
	request api.GetPublicRoadmapEntryRequestObject,
) (api.GetPublicRoadmapEntryResponseObject, error) {
	item, err := h.publicRoadmaps.Entry(ctx, identity.Host(ctx), request.EntryId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetPublicRoadmapEntry200JSONResponse{
		Body:    publicRoadmapItemDTO(item),
		Headers: api.GetPublicRoadmapEntry200ResponseHeaders{CacheControl: &publicRoadmapCache},
	}, nil
}

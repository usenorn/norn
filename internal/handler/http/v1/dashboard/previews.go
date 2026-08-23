package dashboard

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceExecutionPreviews(
	ctx context.Context,
	request api.ListWorkspaceExecutionPreviewsRequestObject,
) (api.ListWorkspaceExecutionPreviewsResponseObject, error) {
	details, err := h.previews.ForExecution(ctx, request.WorkspaceId, request.ExecutionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceExecutionPreviews200JSONResponse(h.previewDetailDTOs(details)), nil
}

func (h *handler) SharePreview(
	ctx context.Context,
	request api.SharePreviewRequestObject,
) (api.SharePreviewResponseObject, error) {
	wanted := service.PreviewShareRequest{Name: request.PreviewName}

	if request.Body != nil {
		wanted.Lifetime = seconds(request.Body.LifetimeSeconds)
		wanted.Passcode = textOf(request.Body.Passcode)
	}

	minted, err := h.previews.Share(ctx, request.WorkspaceId, request.ExecutionId, wanted)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SharePreview201JSONResponse(api.PreviewShareLinkMinted{
		Link: previewShareLinkDTO(minted.Link),
		Url:  minted.URL,
	}), nil
}

func (h *handler) RevokePreviewShareLink(
	ctx context.Context,
	request api.RevokePreviewShareLinkRequestObject,
) (api.RevokePreviewShareLinkResponseObject, error) {
	err := h.previews.RevokeShare(
		ctx, request.WorkspaceId, request.ExecutionId, request.PreviewName, request.ShareLinkId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevokePreviewShareLink204Response{}, nil
}

func (h *handler) RetainWorkspaceExecution(
	ctx context.Context,
	request api.RetainWorkspaceExecutionRequestObject,
) (api.RetainWorkspaceExecutionResponseObject, error) {
	execution, err := h.executions.Retain(
		ctx,
		request.WorkspaceId,
		request.ExecutionId,
		time.Duration(request.Body.LongerSeconds)*time.Second,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RetainWorkspaceExecution200JSONResponse(executionDTO(execution)), nil
}

func (h *handler) AuthorizePreview(
	ctx context.Context,
	request api.AuthorizePreviewRequestObject,
) (api.AuthorizePreviewResponseObject, error) {
	access, err := h.previews.Authorize(
		ctx, request.Params.Host, landing(request.Params.Return),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AuthorizePreview303Response{
		Headers: api.AuthorizePreview303ResponseHeaders{
			Location:     &access.Redirect,
			CacheControl: &noStore,
		},
	}, nil
}

func landing(requested *string) string {
	wanted := textOf(requested)
	if wanted == "" || wanted[0] != '/' || (len(wanted) > 1 && wanted[1] == '/') {
		return "/"
	}

	return wanted
}

func seconds(value *int) time.Duration {
	if value == nil {
		return 0
	}

	return time.Duration(*value) * time.Second
}

func (h *handler) previewDetailDTOs(details []service.PreviewDetail) []api.ExecutionPreviewDetail {
	dtos := make([]api.ExecutionPreviewDetail, 0, len(details))

	for _, detail := range details {
		dtos = append(dtos, api.ExecutionPreviewDetail{
			Preview:    previewDTO(detail.Preview, h.previewCfg.Scheme),
			ShareLinks: previewShareLinkDTOs(detail.Links),
		})
	}

	return dtos
}

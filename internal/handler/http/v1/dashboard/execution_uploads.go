package dashboard

import (
	"context"
	"net/http"

	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const artifactFileFormField = "file"

func (h *handler) UploadExecutionArtifact(
	ctx context.Context,
	request api.UploadExecutionArtifactRequestObject,
) (api.UploadExecutionArtifactResponseObject, error) {
	part, err := request.Body.NextPart()
	if err != nil {
		return newProblem(
			http.StatusBadRequest,
			"the request must carry a single "+artifactFileFormField+" part",
		), nil
	}

	defer func() { _ = part.Close() }()

	if part.FormName() != artifactFileFormField {
		return newProblem(
			http.StatusBadRequest,
			"the request must carry a single "+artifactFileFormField+" part",
		), nil
	}

	receipt, err := h.executionUploads.SaveArtifact(ctx, request.ExecutionId, service.ArtifactUpload{
		Name:        part.FileName(),
		ContentType: part.Header.Get("Content-Type"),
		Body:        part,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UploadExecutionArtifact201JSONResponse(artifactReceiptDTO(receipt)), nil
}

func (h *handler) ListWorkspaceExecutionArtifacts(
	ctx context.Context,
	request api.ListWorkspaceExecutionArtifactsRequestObject,
) (api.ListWorkspaceExecutionArtifactsResponseObject, error) {
	artifacts, err := h.executionUploads.Artifacts(ctx, request.WorkspaceId, request.ExecutionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceExecutionArtifacts200JSONResponse(artifactDTOs(artifacts)), nil
}

func (h *handler) DownloadWorkspaceExecutionArtifact(
	ctx context.Context,
	request api.DownloadWorkspaceExecutionArtifactRequestObject,
) (api.DownloadWorkspaceExecutionArtifactResponseObject, error) {
	target, err := h.executionUploads.ArtifactContent(
		ctx, request.WorkspaceId, request.ExecutionId, request.ArtifactId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DownloadWorkspaceExecutionArtifact303Response{
		Headers: api.DownloadWorkspaceExecutionArtifact303ResponseHeaders{
			Location:     &target,
			CacheControl: &noStore,
		},
	}, nil
}

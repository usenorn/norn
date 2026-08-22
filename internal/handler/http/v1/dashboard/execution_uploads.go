package dashboard

import (
	"context"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const artifactFileFormField = "file"

func (h *handler) UploadExecutionLogs(
	ctx context.Context,
	request api.UploadExecutionLogsRequestObject,
) (api.UploadExecutionLogsResponseObject, error) {
	receipt, err := h.executionUploads.AppendLogs(ctx, request.ExecutionId, service.LogBatch{
		Sequence: request.Body.Sequence,
		Entries:  logEntriesOf(request.Body.Entries),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UploadExecutionLogs202JSONResponse(chunkReceiptDTO(receipt)), nil
}

func (h *handler) UploadExecutionTranscript(
	ctx context.Context,
	request api.UploadExecutionTranscriptRequestObject,
) (api.UploadExecutionTranscriptResponseObject, error) {
	receipt, err := h.executionUploads.AppendTranscript(
		ctx, request.ExecutionId, service.TranscriptBatch{
			Sequence: request.Body.Sequence,
			Entries:  transcriptEntriesOf(request.Body.Entries),
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UploadExecutionTranscript202JSONResponse(chunkReceiptDTO(receipt)), nil
}

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

func (h *handler) GetExecutionStreams(
	ctx context.Context,
	request api.GetExecutionStreamsRequestObject,
) (api.GetExecutionStreamsResponseObject, error) {
	cursors, err := h.executionUploads.Cursors(ctx, request.ExecutionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetExecutionStreams200JSONResponse(streamCursorDTOs(cursors)), nil
}

func (h *handler) ListWorkspaceExecutionLogs(
	ctx context.Context,
	request api.ListWorkspaceExecutionLogsRequestObject,
) (api.ListWorkspaceExecutionLogsResponseObject, error) {
	chunks, err := h.executionUploads.Logs(
		ctx, request.WorkspaceId, request.ExecutionId,
		chunkPageOf(request.Params.After, request.Params.Limit),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceExecutionLogs200JSONResponse(logChunkDTOs(chunks)), nil
}

func (h *handler) ListWorkspaceExecutionTranscript(
	ctx context.Context,
	request api.ListWorkspaceExecutionTranscriptRequestObject,
) (api.ListWorkspaceExecutionTranscriptResponseObject, error) {
	chunks, err := h.executionUploads.Transcript(
		ctx, request.WorkspaceId, request.ExecutionId,
		chunkPageOf(request.Params.After, request.Params.Limit),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceExecutionTranscript200JSONResponse(transcriptChunkDTOs(chunks)), nil
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

func (h *handler) GetWorkspaceExecutionPolicy(
	ctx context.Context,
	request api.GetWorkspaceExecutionPolicyRequestObject,
) (api.GetWorkspaceExecutionPolicyResponseObject, error) {
	policy, err := h.executionUploads.Policy(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceExecutionPolicy200JSONResponse(executionPolicyDTO(policy)), nil
}

func (h *handler) SetWorkspaceExecutionPolicy(
	ctx context.Context,
	request api.SetWorkspaceExecutionPolicyRequestObject,
) (api.SetWorkspaceExecutionPolicyResponseObject, error) {
	policy, err := h.executionUploads.SetPolicy(ctx, entity.WorkspaceExecutionPolicy{
		WorkspaceID:         request.WorkspaceId,
		Telemetry:           entity.TelemetryMode(request.Body.Telemetry),
		UploadRetentionDays: request.Body.UploadRetentionDays,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceExecutionPolicy200JSONResponse(executionPolicyDTO(policy)), nil
}

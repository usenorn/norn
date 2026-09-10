package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceIssueCriteria(
	ctx context.Context,
	request api.ListWorkspaceIssueCriteriaRequestObject,
) (api.ListWorkspaceIssueCriteriaResponseObject, error) {
	criteria, err := h.criteria.List(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueCriteria200JSONResponse{
		Criteria: acceptanceCriterionDTOs(criteria),
	}, nil
}

func (h *handler) RecordWorkspaceIssueEvidence(
	ctx context.Context,
	request api.RecordWorkspaceIssueEvidenceRequestObject,
) (api.RecordWorkspaceIssueEvidenceResponseObject, error) {
	input := service.RecordEvidenceInput{
		CriterionID: request.Body.CriterionId,
		Kind:        entity.EvidenceKind(request.Body.Kind),
		Label:       request.Body.Label,
	}

	if request.Body.Url != nil {
		input.URL = *request.Body.Url
	}

	if request.Body.AttachmentId != nil {
		input.AttachmentID = *request.Body.AttachmentId
	}

	evidence, err := h.criteria.Record(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RecordWorkspaceIssueEvidence201JSONResponse(
		criterionEvidenceDTO(evidence, evidence.CriterionText),
	), nil
}

func (h *handler) DeleteWorkspaceIssueEvidence(
	ctx context.Context,
	request api.DeleteWorkspaceIssueEvidenceRequestObject,
) (api.DeleteWorkspaceIssueEvidenceResponseObject, error) {
	if err := h.criteria.Remove(
		ctx, request.WorkspaceId, request.IssueId, request.EvidenceId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteWorkspaceIssueEvidence204Response{}, nil
}

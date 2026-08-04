package dashboard

import (
	"context"

	api "github.com/usenorn/norn/pkg/http/v1/dashboard"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *handler) ListWorkspaceIssueRelations(
	ctx context.Context,
	request api.ListWorkspaceIssueRelationsRequestObject,
) (api.ListWorkspaceIssueRelationsResponseObject, error) {
	groups, err := h.issueRelations.List(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueRelations200JSONResponse{Groups: issueRelationGroupDTOs(groups)}, nil
}

func (h *handler) AddWorkspaceIssueRelation(
	ctx context.Context,
	request api.AddWorkspaceIssueRelationRequestObject,
) (api.AddWorkspaceIssueRelationResponseObject, error) {
	input := service.AddIssueRelationInput{
		Kind:          entity.IssueRelationView(request.Body.Kind),
		CounterpartID: request.Body.IssueId,
	}

	if request.Body.CloseDuplicate != nil {
		input.CloseDuplicate = *request.Body.CloseDuplicate
	}

	relation, err := h.issueRelations.Add(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AddWorkspaceIssueRelation201JSONResponse(issueRelationDTO(relation)), nil
}

func (h *handler) RemoveWorkspaceIssueRelation(
	ctx context.Context,
	request api.RemoveWorkspaceIssueRelationRequestObject,
) (api.RemoveWorkspaceIssueRelationResponseObject, error) {
	err := h.issueRelations.Remove(ctx, request.WorkspaceId, request.IssueId, request.RelationId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceIssueRelation204Response{}, nil
}

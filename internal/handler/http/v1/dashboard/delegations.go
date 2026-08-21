package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceIssueDelegations(
	ctx context.Context,
	request api.ListWorkspaceIssueDelegationsRequestObject,
) (api.ListWorkspaceIssueDelegationsResponseObject, error) {
	delegations, err := h.delegations.History(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueDelegations200JSONResponse(issueDelegationDTOs(delegations)), nil
}

func (h *handler) DelegateWorkspaceIssue(
	ctx context.Context,
	request api.DelegateWorkspaceIssueRequestObject,
) (api.DelegateWorkspaceIssueResponseObject, error) {
	input := service.DelegateIssueInput{AgentAccountID: request.Body.AgentAccountId}

	if request.Body.Brief != nil {
		input.Brief = *request.Body.Brief
	}

	delegation, err := h.delegations.Delegate(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DelegateWorkspaceIssue201JSONResponse(issueDelegationDTO(delegation)), nil
}

func (h *handler) RecallWorkspaceIssue(
	ctx context.Context,
	request api.RecallWorkspaceIssueRequestObject,
) (api.RecallWorkspaceIssueResponseObject, error) {
	delegation, err := h.delegations.Recall(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RecallWorkspaceIssue200JSONResponse(issueDelegationDTO(delegation)), nil
}

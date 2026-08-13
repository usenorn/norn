package dashboard

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func claimTTL(seconds *int32) time.Duration {
	if seconds == nil {
		return 0
	}

	return time.Duration(*seconds) * time.Second
}

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

func (h *handler) ListWorkspaceDelegationQueue(
	ctx context.Context,
	request api.ListWorkspaceDelegationQueueRequestObject,
) (api.ListWorkspaceDelegationQueueResponseObject, error) {
	queue, err := h.delegations.Queue(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceDelegationQueue200JSONResponse{
		Delegations: delegatedWorkDTOs(queue),
	}, nil
}

func (h *handler) ClaimWorkspaceIssueDelegation(
	ctx context.Context,
	request api.ClaimWorkspaceIssueDelegationRequestObject,
) (api.ClaimWorkspaceIssueDelegationResponseObject, error) {
	claimed, err := h.delegations.Claim(ctx, request.WorkspaceId, request.IssueId, service.ClaimDelegationInput{
		Runner: request.Body.Runner,
		TTL:    claimTTL(request.Body.TtlSeconds),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ClaimWorkspaceIssueDelegation201JSONResponse(delegationClaimDTO(claimed)), nil
}

func (h *handler) HeartbeatWorkspaceIssueDelegationClaim(
	ctx context.Context,
	request api.HeartbeatWorkspaceIssueDelegationClaimRequestObject,
) (api.HeartbeatWorkspaceIssueDelegationClaimResponseObject, error) {
	held, err := h.delegations.Heartbeat(ctx, request.WorkspaceId, request.IssueId, service.HeartbeatDelegationInput{
		Token: request.Body.Token,
		TTL:   claimTTL(request.Body.TtlSeconds),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.HeartbeatWorkspaceIssueDelegationClaim200JSONResponse(delegationClaimDTO(held)), nil
}

func (h *handler) ReleaseWorkspaceIssueDelegationClaim(
	ctx context.Context,
	request api.ReleaseWorkspaceIssueDelegationClaimRequestObject,
) (api.ReleaseWorkspaceIssueDelegationClaimResponseObject, error) {
	if _, err := h.delegations.ReleaseClaim(
		ctx, request.WorkspaceId, request.IssueId, request.Params.Token,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReleaseWorkspaceIssueDelegationClaim204Response{}, nil
}

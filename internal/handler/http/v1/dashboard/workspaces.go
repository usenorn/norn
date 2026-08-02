package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaces(ctx context.Context, _ api.ListWorkspacesRequestObject) (api.ListWorkspacesResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	workspaces, err := h.workspaces.ListForAccount(ctx, accountID)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaces200JSONResponse(workspaceDTOs(workspaces)), nil
}

func (h *handler) CreateWorkspace(ctx context.Context, request api.CreateWorkspaceRequestObject) (api.CreateWorkspaceResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	workspace, err := h.workspaces.Create(ctx, service.CreateWorkspaceInput{
		Slug: request.Body.Slug,
		Name: request.Body.Name,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspace201JSONResponse(workspaceDTO(workspace)), nil
}

func (h *handler) ListWorkspaceMembers(ctx context.Context, request api.ListWorkspaceMembersRequestObject) (api.ListWorkspaceMembersResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	memberships, err := h.workspaces.ListMembers(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceMembers200JSONResponse(membershipDTOs(memberships)), nil
}

func (h *handler) AddWorkspaceMember(ctx context.Context, request api.AddWorkspaceMemberRequestObject) (api.AddWorkspaceMemberResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	membership, err := h.workspaces.AddMember(
		ctx,
		request.WorkspaceId,
		request.Body.AccountId,
		entity.MembershipRole(request.Body.Role),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AddWorkspaceMember201JSONResponse(membershipDTO(membership)), nil
}

func (h *handler) ChangeWorkspaceMemberRole(ctx context.Context, request api.ChangeWorkspaceMemberRoleRequestObject) (api.ChangeWorkspaceMemberRoleResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	membership, err := h.workspaces.ChangeMemberRole(
		ctx,
		request.WorkspaceId,
		request.AccountId,
		entity.MembershipRole(request.Body.Role),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ChangeWorkspaceMemberRole200JSONResponse(membershipDTO(membership)), nil
}

func (h *handler) RemoveWorkspaceMember(ctx context.Context, request api.RemoveWorkspaceMemberRequestObject) (api.RemoveWorkspaceMemberResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	if err := h.workspaces.RemoveMember(ctx, request.WorkspaceId, request.AccountId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceMember204Response{}, nil
}

func (h *handler) GetWorkspaceAuthPolicy(ctx context.Context, request api.GetWorkspaceAuthPolicyRequestObject) (api.GetWorkspaceAuthPolicyResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	policy, err := h.workspaces.AuthPolicy(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceAuthPolicy200JSONResponse(workspaceAuthPolicyDTO(policy)), nil
}

func (h *handler) SetWorkspaceAuthPolicy(ctx context.Context, request api.SetWorkspaceAuthPolicyRequestObject) (api.SetWorkspaceAuthPolicyResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	policy, err := h.workspaces.SetAuthPolicy(ctx, request.WorkspaceId, entity.AuthEnforcement(request.Body.Enforcement))
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceAuthPolicy200JSONResponse(workspaceAuthPolicyDTO(policy)), nil
}

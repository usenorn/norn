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

	input := service.CreateWorkspaceInput{
		Slug: request.Body.Slug,
		Name: request.Body.Name,
	}

	if request.Body.Team != nil {
		input.Team = &service.CreateWorkspaceTeamInput{
			Key:  request.Body.Team.Key,
			Name: request.Body.Team.Name,
		}
	}

	workspace, err := h.workspaces.Create(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspace201JSONResponse(workspaceDTO(workspace)), nil
}

func (h *handler) GetWorkspace(ctx context.Context, request api.GetWorkspaceRequestObject) (api.GetWorkspaceResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	workspace, err := h.workspaces.Get(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspace200JSONResponse(workspaceDTO(workspace)), nil
}

func (h *handler) UpdateWorkspace(ctx context.Context, request api.UpdateWorkspaceRequestObject) (api.UpdateWorkspaceResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	workspace, err := h.workspaces.Update(ctx, request.WorkspaceId, service.UpdateWorkspaceInput{
		Name:          request.Body.Name,
		Timezone:      request.Body.Timezone,
		DefaultTeamID: request.Body.DefaultTeamId,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspace200JSONResponse(workspaceDTO(workspace)), nil
}

func (h *handler) DeleteWorkspace(ctx context.Context, request api.DeleteWorkspaceRequestObject) (api.DeleteWorkspaceResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	workspace, err := h.workspaces.Delete(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteWorkspace200JSONResponse(workspaceDTO(workspace)), nil
}

func (h *handler) RestoreWorkspace(ctx context.Context, request api.RestoreWorkspaceRequestObject) (api.RestoreWorkspaceResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	workspace, err := h.workspaces.Restore(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RestoreWorkspace200JSONResponse(workspaceDTO(workspace)), nil
}

func (h *handler) ListWorkspaceMembers(ctx context.Context, request api.ListWorkspaceMembersRequestObject) (api.ListWorkspaceMembersResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	input := service.ListMembersInput{}

	if request.Params.Query != nil {
		input.Query = *request.Params.Query
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	page, err := h.workspaces.ListMembers(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceMembers200JSONResponse(memberPageDTO(page)), nil
}

func (h *handler) PreviewWorkspaceMemberRemoval(ctx context.Context, request api.PreviewWorkspaceMemberRemovalRequestObject) (api.PreviewWorkspaceMemberRemovalResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	preview, err := h.workspaces.PreviewMemberRemoval(ctx, request.WorkspaceId, request.AccountId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.PreviewWorkspaceMemberRemoval200JSONResponse(memberRemovalPreviewDTO(preview)), nil
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

	if err := h.workspaces.RemoveMember(ctx, request.WorkspaceId, request.AccountId, request.Params.ReassignTo); err != nil {
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

	outcome, err := h.workspaces.SetAuthPolicy(ctx, request.WorkspaceId, entity.AuthEnforcement(request.Body.Enforcement))
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	response := api.SetWorkspaceAuthPolicy200JSONResponse{Policy: workspaceAuthPolicyDTO(outcome.Policy)}

	if len(outcome.RecoveryCodes) > 0 {
		codes := outcome.RecoveryCodes
		response.RecoveryCodes = &codes
	}

	return response, nil
}

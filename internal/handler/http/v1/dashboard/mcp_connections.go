package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) DescribeMCPAuthorization(
	ctx context.Context,
	request api.DescribeMCPAuthorizationRequestObject,
) (api.DescribeMCPAuthorizationResponseObject, error) {
	view, err := h.mcpConnections.DescribeAuthorization(ctx, request.RequestId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	workspaces := make([]api.MCPAuthorizationWorkspace, 0, len(view.Workspaces))

	for _, workspace := range view.Workspaces {
		workspaces = append(workspaces, api.MCPAuthorizationWorkspace{
			Id:   workspace.ID,
			Slug: workspace.Slug,
			Name: workspace.Name,
		})
	}

	return api.DescribeMCPAuthorization200JSONResponse{
		ClientName: view.ClientName,
		Capability: api.MCPCapability(view.Capability),
		Workspaces: workspaces,
	}, nil
}

func (h *handler) ApproveMCPAuthorization(
	ctx context.Context,
	request api.ApproveMCPAuthorizationRequestObject,
) (api.ApproveMCPAuthorizationResponseObject, error) {
	decision, err := h.mcpConnections.Approve(ctx, request.RequestId, service.ApproveMCPAuthorizationInput{
		AllWorkspaces: request.Body.AllWorkspaces,
		WorkspaceIDs:  request.Body.WorkspaceIds,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ApproveMCPAuthorization200JSONResponse{RedirectTo: decision.RedirectTo}, nil
}

func (h *handler) DenyMCPAuthorization(
	ctx context.Context,
	request api.DenyMCPAuthorizationRequestObject,
) (api.DenyMCPAuthorizationResponseObject, error) {
	decision, err := h.mcpConnections.Deny(ctx, request.RequestId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DenyMCPAuthorization200JSONResponse{RedirectTo: decision.RedirectTo}, nil
}

func (h *handler) ListMCPConnections(
	ctx context.Context,
	_ api.ListMCPConnectionsRequestObject,
) (api.ListMCPConnectionsResponseObject, error) {
	connections, err := h.mcpConnections.List(ctx)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dtos := make([]api.MCPConnection, 0, len(connections))

	for _, connection := range connections {
		dtos = append(dtos, mcpConnectionDTO(connection))
	}

	return api.ListMCPConnections200JSONResponse(dtos), nil
}

func (h *handler) NarrowMCPConnection(
	ctx context.Context,
	request api.NarrowMCPConnectionRequestObject,
) (api.NarrowMCPConnectionResponseObject, error) {
	input := service.NarrowMCPConnectionInput{}

	if request.Body.Capability != nil {
		capability := entity.MCPCapability(*request.Body.Capability)
		input.Capability = &capability
	}

	if request.Body.Grants != nil {
		grants := apiTokenGrants(*request.Body.Grants)
		input.Grants = &grants
	}

	connection, err := h.mcpConnections.Narrow(ctx, request.ConnectionId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.NarrowMCPConnection200JSONResponse(mcpConnectionDTO(connection)), nil
}

func (h *handler) RevokeMCPConnection(
	ctx context.Context,
	request api.RevokeMCPConnectionRequestObject,
) (api.RevokeMCPConnectionResponseObject, error) {
	if err := h.mcpConnections.Revoke(ctx, request.ConnectionId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevokeMCPConnection204Response{}, nil
}

func (h *handler) ListWorkspaceMCPConnections(
	ctx context.Context,
	request api.ListWorkspaceMCPConnectionsRequestObject,
) (api.ListWorkspaceMCPConnectionsResponseObject, error) {
	connections, err := h.mcpConnections.ListForWorkspace(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dtos := make([]api.WorkspaceMCPConnection, 0, len(connections))

	for _, owned := range connections {
		dtos = append(dtos, api.WorkspaceMCPConnection{
			Connection: mcpConnectionDTO(owned.Connection),
			OwnerName:  owned.OwnerName,
			OwnerEmail: owned.OwnerEmail,
		})
	}

	return api.ListWorkspaceMCPConnections200JSONResponse(dtos), nil
}

func (h *handler) RevokeWorkspaceMCPConnection(
	ctx context.Context,
	request api.RevokeWorkspaceMCPConnectionRequestObject,
) (api.RevokeWorkspaceMCPConnectionResponseObject, error) {
	if err := h.mcpConnections.RevokeInWorkspace(ctx, request.WorkspaceId, request.ConnectionId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevokeWorkspaceMCPConnection204Response{}, nil
}

func mcpConnectionDTO(connection entity.MCPConnection) api.MCPConnection {
	dto := api.MCPConnection{
		Id:                connection.ID,
		ClientName:        connection.ClientName,
		Capability:        api.MCPCapability(connection.Capability()),
		FollowsMembership: connection.FollowsMembership(),
		Grants:            apiTokenGrantDTOs(connection.Grants),
		CreatedAt:         connection.CreatedAt,
	}

	dto.LastUsedAt = connection.LastUsedAt

	return dto
}

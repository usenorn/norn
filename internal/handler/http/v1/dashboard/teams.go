package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceTeams(ctx context.Context, request api.ListWorkspaceTeamsRequestObject) (api.ListWorkspaceTeamsResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	var status entity.TeamStatus
	if request.Params.Status != nil {
		status = entity.TeamStatus(*request.Params.Status)
	}

	teams, err := h.teams.List(ctx, request.WorkspaceId, status)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceTeams200JSONResponse(teamDTOs(teams)), nil
}

func (h *handler) CreateWorkspaceTeam(ctx context.Context, request api.CreateWorkspaceTeamRequestObject) (api.CreateWorkspaceTeamResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	input := service.CreateTeamInput{
		WorkspaceID: request.WorkspaceId,
		Key:         request.Body.Key,
		Name:        request.Body.Name,
	}

	if request.Body.Visibility != nil {
		input.Visibility = entity.TeamVisibility(*request.Body.Visibility)
	}

	team, err := h.teams.Create(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceTeam201JSONResponse(teamDTO(team)), nil
}

func (h *handler) GetWorkspaceTeam(ctx context.Context, request api.GetWorkspaceTeamRequestObject) (api.GetWorkspaceTeamResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	team, err := h.teams.Get(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceTeam200JSONResponse(teamDTO(team)), nil
}

func (h *handler) UpdateWorkspaceTeam(ctx context.Context, request api.UpdateWorkspaceTeamRequestObject) (api.UpdateWorkspaceTeamResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	input := service.UpdateTeamInput{
		Name:        request.Body.Name,
		Description: request.Body.Description,
		Icon:        request.Body.Icon,
	}

	if request.Body.IconColor != nil {
		color := entity.TeamColor(*request.Body.IconColor)
		input.IconColor = &color
	}

	if request.Body.Estimation != nil {
		estimation := entity.TeamEstimation(*request.Body.Estimation)
		input.Estimation = &estimation
	}

	if request.Body.Visibility != nil {
		visibility := entity.TeamVisibility(*request.Body.Visibility)
		input.Visibility = &visibility
	}

	team, err := h.teams.Update(ctx, request.WorkspaceId, request.TeamId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceTeam200JSONResponse(teamDTO(team)), nil
}

func (h *handler) ArchiveWorkspaceTeam(ctx context.Context, request api.ArchiveWorkspaceTeamRequestObject) (api.ArchiveWorkspaceTeamResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	team, err := h.teams.Archive(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ArchiveWorkspaceTeam200JSONResponse(teamDTO(team)), nil
}

func (h *handler) UnarchiveWorkspaceTeam(ctx context.Context, request api.UnarchiveWorkspaceTeamRequestObject) (api.UnarchiveWorkspaceTeamResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	team, err := h.teams.Unarchive(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnarchiveWorkspaceTeam200JSONResponse(teamDTO(team)), nil
}

func (h *handler) ListWorkspaceTeamMembers(ctx context.Context, request api.ListWorkspaceTeamMembersRequestObject) (api.ListWorkspaceTeamMembersResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	members, err := h.teams.ListMembers(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceTeamMembers200JSONResponse(teamMemberDTOs(members)), nil
}

func (h *handler) AddWorkspaceTeamMember(ctx context.Context, request api.AddWorkspaceTeamMemberRequestObject) (api.AddWorkspaceTeamMemberResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	member, err := h.teams.AddMember(ctx, request.WorkspaceId, request.TeamId, request.Body.AccountId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AddWorkspaceTeamMember201JSONResponse(teamMemberDTO(member)), nil
}

func (h *handler) RemoveWorkspaceTeamMember(ctx context.Context, request api.RemoveWorkspaceTeamMemberRequestObject) (api.RemoveWorkspaceTeamMemberResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	if err := h.teams.RemoveMember(ctx, request.WorkspaceId, request.TeamId, request.AccountId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceTeamMember204Response{}, nil
}

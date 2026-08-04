package dashboard

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceProjects(
	ctx context.Context,
	request api.ListWorkspaceProjectsRequestObject,
) (api.ListWorkspaceProjectsResponseObject, error) {
	input := service.ListProjectsInput{
		Archived: request.Params.Archived != nil && *request.Params.Archived,
		Mine:     request.Params.Mine != nil && *request.Params.Mine,
	}

	if request.Params.State != nil {
		input.State = entity.ProjectState(*request.Params.State)
	}

	views, err := h.projects.List(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceProjects200JSONResponse(projectDTOs(views)), nil
}

func (h *handler) CreateWorkspaceProject(
	ctx context.Context,
	request api.CreateWorkspaceProjectRequestObject,
) (api.CreateWorkspaceProjectResponseObject, error) {
	input := service.CreateProjectInput{
		WorkspaceID:   request.WorkspaceId,
		Slug:          request.Body.Slug,
		Name:          request.Body.Name,
		LeadAccountID: request.Body.LeadAccountId,
	}

	if request.Body.Description != nil {
		input.Description = *request.Body.Description
	}

	if request.Body.TargetOn != nil {
		input.TargetOn = request.Body.TargetOn.Format(time.DateOnly)
	}

	view, err := h.projects.Create(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceProject201JSONResponse(projectDTO(view)), nil
}

func (h *handler) GetWorkspaceProject(
	ctx context.Context,
	request api.GetWorkspaceProjectRequestObject,
) (api.GetWorkspaceProjectResponseObject, error) {
	view, err := h.projects.Get(ctx, request.WorkspaceId, request.ProjectId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceProject200JSONResponse(projectDTO(view)), nil
}

func (h *handler) UpdateWorkspaceProject(
	ctx context.Context,
	request api.UpdateWorkspaceProjectRequestObject,
) (api.UpdateWorkspaceProjectResponseObject, error) {
	if request.Body.State != nil {
		view, err := h.projects.SetState(
			ctx,
			request.WorkspaceId,
			request.ProjectId,
			entity.ProjectState(*request.Body.State),
		)
		if err != nil {
			if problem, ok := problemFor(err); ok {
				return problem, nil
			}

			return nil, err
		}

		return api.UpdateWorkspaceProject200JSONResponse(projectDTO(view)), nil
	}

	input := service.UpdateProjectInput{
		Name:          request.Body.Name,
		Description:   request.Body.Description,
		LeadAccountID: request.Body.LeadAccountId,
	}

	if request.Body.TargetOn != nil {
		target := request.Body.TargetOn.Format(time.DateOnly)
		input.TargetOn = &target
	}

	if request.Body.Clear != nil {
		for _, field := range *request.Body.Clear {
			input.Clear = append(input.Clear, string(field))
		}
	}

	view, err := h.projects.Update(ctx, request.WorkspaceId, request.ProjectId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceProject200JSONResponse(projectDTO(view)), nil
}

func (h *handler) DeleteWorkspaceProject(
	ctx context.Context,
	request api.DeleteWorkspaceProjectRequestObject,
) (api.DeleteWorkspaceProjectResponseObject, error) {
	if err := h.projects.Remove(ctx, request.WorkspaceId, request.ProjectId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteWorkspaceProject204Response{}, nil
}

func (h *handler) ArchiveWorkspaceProject(
	ctx context.Context,
	request api.ArchiveWorkspaceProjectRequestObject,
) (api.ArchiveWorkspaceProjectResponseObject, error) {
	view, err := h.projects.Archive(ctx, request.WorkspaceId, request.ProjectId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ArchiveWorkspaceProject200JSONResponse(projectDTO(view)), nil
}

func (h *handler) UnarchiveWorkspaceProject(
	ctx context.Context,
	request api.UnarchiveWorkspaceProjectRequestObject,
) (api.UnarchiveWorkspaceProjectResponseObject, error) {
	view, err := h.projects.Unarchive(ctx, request.WorkspaceId, request.ProjectId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnarchiveWorkspaceProject200JSONResponse(projectDTO(view)), nil
}

func (h *handler) ListWorkspaceProjectMembers(
	ctx context.Context,
	request api.ListWorkspaceProjectMembersRequestObject,
) (api.ListWorkspaceProjectMembersResponseObject, error) {
	members, err := h.projects.ListMembers(ctx, request.WorkspaceId, request.ProjectId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceProjectMembers200JSONResponse(projectMemberDTOs(members)), nil
}

func (h *handler) AddWorkspaceProjectMember(
	ctx context.Context,
	request api.AddWorkspaceProjectMemberRequestObject,
) (api.AddWorkspaceProjectMemberResponseObject, error) {
	member, err := h.projects.AddMember(
		ctx,
		request.WorkspaceId,
		request.ProjectId,
		request.Body.AccountId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AddWorkspaceProjectMember201JSONResponse(projectMemberDTO(member)), nil
}

func (h *handler) RemoveWorkspaceProjectMember(
	ctx context.Context,
	request api.RemoveWorkspaceProjectMemberRequestObject,
) (api.RemoveWorkspaceProjectMemberResponseObject, error) {
	if err := h.projects.RemoveMember(
		ctx,
		request.WorkspaceId,
		request.ProjectId,
		request.AccountId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceProjectMember204Response{}, nil
}

func (h *handler) ListWorkspaceProjectStatus(
	ctx context.Context,
	request api.ListWorkspaceProjectStatusRequestObject,
) (api.ListWorkspaceProjectStatusResponseObject, error) {
	updates, err := h.projects.ListStatus(ctx, request.WorkspaceId, request.ProjectId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceProjectStatus200JSONResponse(projectStatusDTOs(updates)), nil
}

func (h *handler) PostWorkspaceProjectStatus(
	ctx context.Context,
	request api.PostWorkspaceProjectStatusRequestObject,
) (api.PostWorkspaceProjectStatusResponseObject, error) {
	update, err := h.projects.PostStatus(
		ctx,
		request.WorkspaceId,
		request.ProjectId,
		service.PostProjectStatusInput{
			Health: entity.ProjectHealth(request.Body.Health),
			Body:   request.Body.Body,
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.PostWorkspaceProjectStatus201JSONResponse(projectStatusDTO(update)), nil
}

func (h *handler) ListWorkspaceProjectActivity(
	ctx context.Context,
	request api.ListWorkspaceProjectActivityRequestObject,
) (api.ListWorkspaceProjectActivityResponseObject, error) {
	page, err := h.projects.Activity(
		ctx, request.WorkspaceId, request.ProjectId,
		activityInput(
			request.Params.Limit,
			request.Params.Cursor,
			(*api.ListWorkspaceIssueActivityParamsOrder)(request.Params.Order),
		),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceProjectActivity200JSONResponse(activityPageDTO(page)), nil
}

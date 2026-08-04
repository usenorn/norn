package dashboard

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceLabels(
	ctx context.Context,
	request api.ListWorkspaceLabelsRequestObject,
) (api.ListWorkspaceLabelsResponseObject, error) {
	labels, err := h.labels.List(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceLabels200JSONResponse(labelDTOs(labels)), nil
}

func (h *handler) CreateWorkspaceLabel(
	ctx context.Context,
	request api.CreateWorkspaceLabelRequestObject,
) (api.CreateWorkspaceLabelResponseObject, error) {
	input := service.CreateLabelInput{
		WorkspaceID: request.WorkspaceId,
		Name:        request.Body.Name,
		Color:       entity.LabelColor(request.Body.Color),
	}

	if request.Body.TeamId != nil {
		input.TeamID = *request.Body.TeamId
	}

	if request.Body.GroupId != nil {
		input.GroupID = *request.Body.GroupId
	}

	label, err := h.labels.Create(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceLabel201JSONResponse(labelDTO(label)), nil
}

func (h *handler) UpdateWorkspaceLabel(
	ctx context.Context,
	request api.UpdateWorkspaceLabelRequestObject,
) (api.UpdateWorkspaceLabelResponseObject, error) {
	input := service.UpdateLabelInput{Name: request.Body.Name}

	if request.Body.Color != nil {
		color := entity.LabelColor(*request.Body.Color)
		input.Color = &color
	}

	if request.Body.GroupId != nil {
		groupID := uuid.Nil
		if *request.Body.GroupId != uuid.Nil {
			groupID = *request.Body.GroupId
		}

		input.GroupID = &groupID
	}

	label, err := h.labels.Update(ctx, request.WorkspaceId, request.LabelId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceLabel200JSONResponse(labelDTO(label)), nil
}

func (h *handler) RemoveWorkspaceLabel(
	ctx context.Context,
	request api.RemoveWorkspaceLabelRequestObject,
) (api.RemoveWorkspaceLabelResponseObject, error) {
	err := h.labels.Remove(
		ctx,
		request.WorkspaceId,
		request.LabelId,
		int(request.Params.AcknowledgedIssues),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceLabel204Response{}, nil
}

func (h *handler) GetWorkspaceLabelUsage(
	ctx context.Context,
	request api.GetWorkspaceLabelUsageRequestObject,
) (api.GetWorkspaceLabelUsageResponseObject, error) {
	usage, err := h.labels.Usage(ctx, request.WorkspaceId, request.LabelId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceLabelUsage200JSONResponse(labelUsageDTO(usage)), nil
}

func (h *handler) MergeWorkspaceLabel(
	ctx context.Context,
	request api.MergeWorkspaceLabelRequestObject,
) (api.MergeWorkspaceLabelResponseObject, error) {
	label, err := h.labels.Merge(ctx, request.WorkspaceId, request.LabelId, request.Body.IntoLabelId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.MergeWorkspaceLabel200JSONResponse(labelDTO(label)), nil
}

func (h *handler) ListWorkspaceLabelGroups(
	ctx context.Context,
	request api.ListWorkspaceLabelGroupsRequestObject,
) (api.ListWorkspaceLabelGroupsResponseObject, error) {
	groups, err := h.labels.Groups(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceLabelGroups200JSONResponse(labelGroupDTOs(groups)), nil
}

func (h *handler) CreateWorkspaceLabelGroup(
	ctx context.Context,
	request api.CreateWorkspaceLabelGroupRequestObject,
) (api.CreateWorkspaceLabelGroupResponseObject, error) {
	group, err := h.labels.CreateGroup(ctx, request.WorkspaceId, request.Body.Name)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceLabelGroup201JSONResponse(labelGroupDTO(group)), nil
}

func (h *handler) RenameWorkspaceLabelGroup(
	ctx context.Context,
	request api.RenameWorkspaceLabelGroupRequestObject,
) (api.RenameWorkspaceLabelGroupResponseObject, error) {
	group, err := h.labels.RenameGroup(ctx, request.WorkspaceId, request.GroupId, request.Body.Name)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RenameWorkspaceLabelGroup200JSONResponse(labelGroupDTO(group)), nil
}

func (h *handler) RemoveWorkspaceLabelGroup(
	ctx context.Context,
	request api.RemoveWorkspaceLabelGroupRequestObject,
) (api.RemoveWorkspaceLabelGroupResponseObject, error) {
	if err := h.labels.RemoveGroup(ctx, request.WorkspaceId, request.GroupId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceLabelGroup204Response{}, nil
}

func (h *handler) SetWorkspaceIssueLabels(
	ctx context.Context,
	request api.SetWorkspaceIssueLabelsRequestObject,
) (api.SetWorkspaceIssueLabelsResponseObject, error) {
	labels, err := h.issues.SetLabels(ctx, request.WorkspaceId, request.IssueId, service.SetIssueLabelsInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		LabelIDs:        request.Body.LabelIds,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceIssueLabels200JSONResponse(labelDTOs(labels)), nil
}

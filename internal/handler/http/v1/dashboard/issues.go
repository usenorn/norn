package dashboard

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"

	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceIssues(
	ctx context.Context,
	request api.ListWorkspaceIssuesRequestObject,
) (api.ListWorkspaceIssuesResponseObject, error) {
	input := service.ListIssuesInput{}

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	input.TeamID = request.Params.TeamId

	if request.Params.Status != nil {
		for _, status := range *request.Params.Status {
			input.Statuses = append(input.Statuses, entity.IssueStatus(status))
		}
	}

	page, err := h.issues.List(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssues200JSONResponse(issuePageDTO(page)), nil
}

func (h *handler) CreateWorkspaceIssue(
	ctx context.Context,
	request api.CreateWorkspaceIssueRequestObject,
) (api.CreateWorkspaceIssueResponseObject, error) {
	issue, err := h.issues.Create(ctx, service.CreateIssueInput{
		WorkspaceID: request.WorkspaceId,
		TeamID:      request.Body.TeamId,
		Title:       request.Body.Title,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceIssue201JSONResponse(issueDTO(issue)), nil
}

func (h *handler) GetWorkspaceIssue(
	ctx context.Context,
	request api.GetWorkspaceIssueRequestObject,
) (api.GetWorkspaceIssueResponseObject, error) {
	issue, err := h.issues.Get(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceIssue200JSONResponse(issueDTO(issue)), nil
}

func (h *handler) UpdateWorkspaceIssue(
	ctx context.Context,
	request api.UpdateWorkspaceIssueRequestObject,
) (api.UpdateWorkspaceIssueResponseObject, error) {
	input := service.UpdateIssueInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		Title:           request.Body.Title,
		StateID:         request.Body.StateId,
		Description:     request.Body.Description,
		AssigneeID:      request.Body.AssigneeId,
	}

	if request.Body.Priority != nil {
		priority := entity.IssuePriority(*request.Body.Priority)
		input.Priority = &priority
	}

	if request.Body.Estimate != nil {
		estimate := int(*request.Body.Estimate)
		input.Estimate = &estimate
	}

	if request.Body.DueOn != nil {
		due := request.Body.DueOn.Format(time.DateOnly)
		input.DueOn = &due
	}

	if request.Body.Clear != nil {
		for _, field := range *request.Body.Clear {
			input.Clear = append(input.Clear, string(field))
		}
	}

	issue, err := h.issues.Update(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceIssue200JSONResponse(issueDTO(issue)), nil
}

func (h *handler) GetWorkspaceIssueProgress(
	ctx context.Context,
	request api.GetWorkspaceIssueProgressRequestObject,
) (api.GetWorkspaceIssueProgressResponseObject, error) {
	progress, err := h.issues.Progress(ctx, request.WorkspaceId, request.Params.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceIssueProgress200JSONResponse(issueProgressDTO(progress)), nil
}

func (h *handler) ListWorkspaceIssueActivity(
	ctx context.Context,
	request api.ListWorkspaceIssueActivityRequestObject,
) (api.ListWorkspaceIssueActivityResponseObject, error) {
	input := service.ListIssueActivityInput{}

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	page, err := h.issues.Activity(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueActivity200JSONResponse(issueActivityPageDTO(page)), nil
}

func (h *handler) MoveWorkspaceIssueToTeam(
	ctx context.Context,
	request api.MoveWorkspaceIssueToTeamRequestObject,
) (api.MoveWorkspaceIssueToTeamResponseObject, error) {
	input := service.MoveIssueInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		TeamID:          request.Body.TeamId,
	}

	if request.Body.AcknowledgeLabelLoss != nil {
		input.AcknowledgeLabelLoss = *request.Body.AcknowledgeLabelLoss
	}

	issue, err := h.issues.MoveToTeam(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.MoveWorkspaceIssueToTeam200JSONResponse(issueDTO(issue)), nil
}

func (h *handler) SetWorkspaceIssueStatus(
	ctx context.Context,
	request api.SetWorkspaceIssueStatusRequestObject,
) (api.SetWorkspaceIssueStatusResponseObject, error) {
	issue, err := h.issues.SetStatus(ctx, request.WorkspaceId, request.IssueId, service.SetIssueStatusInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		Status:          entity.IssueStatus(request.Body.Status),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceIssueStatus200JSONResponse(issueDTO(issue)), nil
}

func (h *handler) GetWorkspaceIssueByReference(
	ctx context.Context,
	request api.GetWorkspaceIssueByReferenceRequestObject,
) (api.GetWorkspaceIssueByReferenceResponseObject, error) {
	issue, err := h.issues.GetByReference(ctx, request.WorkspaceId, request.Reference)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceIssueByReference200JSONResponse(issueDTO(issue)), nil
}

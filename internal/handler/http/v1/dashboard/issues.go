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
	input.CycleID = request.Params.CycleId
	input.ProjectID = request.Params.ProjectId

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

func (h *handler) QueryWorkspaceIssues(
	ctx context.Context,
	request api.QueryWorkspaceIssuesRequestObject,
) (api.QueryWorkspaceIssuesResponseObject, error) {
	input := service.QueryIssuesInput{
		Text: textOf(request.Body.Text), Filter: issueFilterFrom(request.Body.Filter)}

	if request.Body.Sort != nil {
		input.Sort = issueSortFrom(*request.Body.Sort)
	}

	if request.Body.GroupBy != nil {
		input.GroupBy = entity.IssueGroupBy(*request.Body.GroupBy)
	}

	if request.Body.Limit != nil {
		input.Limit = int(*request.Body.Limit)
	}

	if request.Body.PerGroup != nil {
		input.PerGroup = int(*request.Body.PerGroup)
	}

	if request.Body.Cursor != nil {
		input.Cursor = *request.Body.Cursor
	}

	result, err := h.issues.Query(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.QueryWorkspaceIssues200JSONResponse(
		issueQueryResultDTO(result, input.GroupBy != ""),
	), nil
}

func (h *handler) CreateWorkspaceIssue(
	ctx context.Context,
	request api.CreateWorkspaceIssueRequestObject,
) (api.CreateWorkspaceIssueResponseObject, error) {
	input := service.CreateIssueInput{
		WorkspaceID: request.WorkspaceId,
		TeamID:      request.Body.TeamId,
		Title:       request.Body.Title,
		Reasoning:   agentReasoningFrom(request.Body.Reasoning),
	}

	if request.Body.Description != nil {
		input.Description = *request.Body.Description
	}

	if request.Body.Priority != nil {
		input.Priority = entity.IssuePriority(*request.Body.Priority)
	}

	if request.Body.AssigneeId != nil {
		input.AssigneeAccountID = *request.Body.AssigneeId
	}

	if request.Body.Estimate != nil {
		input.Estimate = int(*request.Body.Estimate)
	}

	if request.Body.DueOn != nil {
		input.DueOn = request.Body.DueOn.Format(time.DateOnly)
	}

	if request.Body.StateId != nil {
		input.StateID = *request.Body.StateId
	}

	if request.Body.ProjectId != nil {
		input.ProjectID = *request.Body.ProjectId
	}

	if request.Body.LabelIds != nil {
		input.LabelIDs = *request.Body.LabelIds
	}

	issue, err := h.issues.Create(ctx, input)
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
		AcknowledgeOpenChildren: request.Body.AcknowledgeOpenChildren != nil &&
			*request.Body.AcknowledgeOpenChildren,
		AcknowledgeUnprovenChecks: request.Body.AcknowledgeUnprovenChecks != nil &&
			*request.Body.AcknowledgeUnprovenChecks,
		Reasoning:   agentReasoningFrom(request.Body.Reasoning),
		Title:       request.Body.Title,
		StateID:     request.Body.StateId,
		Description: request.Body.Description,
		AssigneeID:  request.Body.AssigneeId,
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

	input.CycleID = request.Body.CycleId
	input.ProjectID = request.Body.ProjectId
	input.AfterIssueID = request.Body.AfterIssueId
	input.BeforeIssueID = request.Body.BeforeIssueId

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
	progress, err := h.issues.Progress(ctx, request.WorkspaceId, service.ProgressInput{
		TeamID:    request.Params.TeamId,
		CycleID:   request.Params.CycleId,
		ProjectID: request.Params.ProjectId,
	})
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
	page, err := h.issues.Activity(
		ctx, request.WorkspaceId, request.IssueId,
		activityInput(request.Params.Limit, request.Params.Cursor, request.Params.Order),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueActivity200JSONResponse(activityPageDTO(page)), nil
}

func activityInput(limit *int32, cursor *string, order *api.ListWorkspaceIssueActivityParamsOrder) service.ListActivityInput {
	input := service.ListActivityInput{}

	if limit != nil {
		input.Limit = int(*limit)
	}

	if cursor != nil {
		input.Cursor = *cursor
	}

	if order != nil {
		input.Order = entity.ActivityOrder(*order)
	}

	return input
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

func (h *handler) SetWorkspaceIssueParent(
	ctx context.Context,
	request api.SetWorkspaceIssueParentRequestObject,
) (api.SetWorkspaceIssueParentResponseObject, error) {
	issue, err := h.issues.SetParent(ctx, request.WorkspaceId, request.IssueId, service.SetIssueParentInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		ParentID:        request.Body.ParentId,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceIssueParent200JSONResponse(issueDTO(issue)), nil
}

func (h *handler) ListWorkspaceIssueChildren(
	ctx context.Context,
	request api.ListWorkspaceIssueChildrenRequestObject,
) (api.ListWorkspaceIssueChildrenResponseObject, error) {
	children, progress, err := h.issues.Children(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueChildren200JSONResponse{
		Issues:   issueDTOs(children),
		Progress: issueProgressDTO(progress),
	}, nil
}

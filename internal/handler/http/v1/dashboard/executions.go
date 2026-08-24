package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceIssueExecutions(
	ctx context.Context,
	request api.ListWorkspaceIssueExecutionsRequestObject,
) (api.ListWorkspaceIssueExecutionsResponseObject, error) {
	executions, err := h.executions.ListByIssue(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueExecutions200JSONResponse(executionDTOs(executions)), nil
}

func (h *handler) ListWorkspaceExecutions(
	ctx context.Context,
	request api.ListWorkspaceExecutionsRequestObject,
) (api.ListWorkspaceExecutionsResponseObject, error) {
	page := entity.ExecutionPage{}

	if request.Params.State != nil {
		for _, state := range *request.Params.State {
			page.States = append(page.States, entity.ExecutionState(state))
		}
	}

	if request.Params.Limit != nil {
		page.Limit = *request.Params.Limit
	}

	listings, err := h.executions.List(ctx, request.WorkspaceId, page)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceExecutions200JSONResponse(executionSummaryDTOs(listings)), nil
}

func (h *handler) GetWorkspaceExecution(
	ctx context.Context,
	request api.GetWorkspaceExecutionRequestObject,
) (api.GetWorkspaceExecutionResponseObject, error) {
	detail, err := h.executions.Get(ctx, request.WorkspaceId, request.ExecutionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	previews := previewDTOs(detail.Previews, h.previewCfg.Scheme)
	services := executionServiceDTOs(detail.Services)

	return api.GetWorkspaceExecution200JSONResponse(api.ExecutionDetail{
		Execution: executionDTO(detail.Execution),
		Timeline:  executionEventDTOs(detail.Timeline),
		Changeset: changeSetOrNothing(detail.Execution.ID, detail.ChangeSet),
		Previews:  &previews,
		Services:  &services,
		Runner:    executionRunnerDTO(detail.Machine),
	}), nil
}

func (h *handler) ListWorkspaceExecutionServices(
	ctx context.Context,
	request api.ListWorkspaceExecutionServicesRequestObject,
) (api.ListWorkspaceExecutionServicesResponseObject, error) {
	services, err := h.executionServices.ForExecution(
		ctx, request.WorkspaceId, request.ExecutionId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceExecutionServices200JSONResponse(
		executionServiceDTOs(services),
	), nil
}

func changeSetOrNothing(
	executionID string,
	changeset entity.ExecutionChangeSet,
) *api.ExecutionChangeSet {
	if changeset.Empty() {
		return nil
	}

	dto := changeSetDTO(executionID, changeset)

	return &dto
}

func (h *handler) GetWorkspaceIssueChangeSet(
	ctx context.Context,
	request api.GetWorkspaceIssueChangeSetRequestObject,
) (api.GetWorkspaceIssueChangeSetResponseObject, error) {
	changeset, err := h.changesets.ForIssue(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceIssueChangeSet200JSONResponse(issueChangeSetDTO(changeset)), nil
}

func (h *handler) ListWorkspaceExecutionTimeline(
	ctx context.Context,
	request api.ListWorkspaceExecutionTimelineRequestObject,
) (api.ListWorkspaceExecutionTimelineResponseObject, error) {
	page := entity.ExecutionTimelinePage{}

	if request.Params.After != nil {
		page.After = *request.Params.After
	}

	if request.Params.Limit != nil {
		page.Limit = *request.Params.Limit
	}

	events, err := h.executions.Timeline(ctx, request.WorkspaceId, request.ExecutionId, page)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceExecutionTimeline200JSONResponse(executionEventDTOs(events)), nil
}

func (h *handler) CancelWorkspaceExecution(
	ctx context.Context,
	request api.CancelWorkspaceExecutionRequestObject,
) (api.CancelWorkspaceExecutionResponseObject, error) {
	var reason string

	if request.Body != nil {
		reason = textOf(request.Body.Reason)
	}

	execution, err := h.executions.Cancel(ctx, request.WorkspaceId, request.ExecutionId, reason)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CancelWorkspaceExecution200JSONResponse(executionDTO(execution)), nil
}

func (h *handler) RestartWorkspaceExecution(
	ctx context.Context,
	request api.RestartWorkspaceExecutionRequestObject,
) (api.RestartWorkspaceExecutionResponseObject, error) {
	execution, err := h.executions.Restart(ctx, request.WorkspaceId, request.ExecutionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RestartWorkspaceExecution201JSONResponse(executionDTO(execution)), nil
}

func (h *handler) ResumeWorkspaceExecution(
	ctx context.Context,
	request api.ResumeWorkspaceExecutionRequestObject,
) (api.ResumeWorkspaceExecutionResponseObject, error) {
	execution, err := h.executions.Resume(
		ctx, request.WorkspaceId, request.ExecutionId, request.Body.Feedback,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ResumeWorkspaceExecution200JSONResponse(executionDTO(execution)), nil
}

func (h *handler) ApproveWorkspaceExecution(
	ctx context.Context,
	request api.ApproveWorkspaceExecutionRequestObject,
) (api.ApproveWorkspaceExecutionResponseObject, error) {
	execution, err := h.executions.Approve(ctx, request.WorkspaceId, request.ExecutionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ApproveWorkspaceExecution200JSONResponse(executionDTO(execution)), nil
}

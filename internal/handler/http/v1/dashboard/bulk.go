package dashboard

import (
	"context"
	"net/http"

	api "github.com/usenorn/norn/pkg/http/v1/dashboard"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *handler) ApplyWorkspaceIssueBulkChange(
	ctx context.Context,
	request api.ApplyWorkspaceIssueBulkChangeRequestObject,
) (api.ApplyWorkspaceIssueBulkChangeResponseObject, error) {
	input := service.ApplyBulkInput{
		Change: bulkChangeFrom(request.Body.Change),
		Set:    entity.BulkSet{},
	}

	if request.Body.IssueIds != nil {
		input.Set.IssueIDs = *request.Body.IssueIds
	}

	if request.Body.Filter != nil {
		filter := entity.BulkFilter{TeamID: request.Body.Filter.TeamId}

		if request.Body.Filter.Status != nil {
			for _, status := range *request.Body.Filter.Status {
				filter.Statuses = append(filter.Statuses, entity.IssueStatus(status))
			}
		}

		input.Set.Filter = &filter
	}

	result, err := h.bulkOperations.Apply(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	body := bulkResultDTO(*result.Action, result.Outcomes)

	if result.Action.Status == entity.BulkActionComplete {
		return api.ApplyWorkspaceIssueBulkChange200JSONResponse(body), nil
	}

	return api.ApplyWorkspaceIssueBulkChange202JSONResponse(body), nil
}

func (h *handler) GetWorkspaceBulkAction(
	ctx context.Context,
	request api.GetWorkspaceBulkActionRequestObject,
) (api.GetWorkspaceBulkActionResponseObject, error) {
	action, outcomes, err := h.bulkOperations.Get(ctx, request.WorkspaceId, request.BulkActionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceBulkAction200JSONResponse(bulkResultDTO(action, outcomes)), nil
}

func bulkChangeFrom(body api.BulkChange) entity.BulkChange {
	change := entity.BulkChange{
		StateID:    body.StateId,
		AssigneeID: body.AssigneeId,
		AddLabelID: body.AddLabelId,
	}

	if body.ClearAssignee != nil {
		change.ClearAssignee = *body.ClearAssignee
	}

	if body.Priority != nil {
		priority := entity.IssuePriority(*body.Priority)
		change.Priority = &priority
	}

	if body.Status != nil {
		status := entity.IssueStatus(*body.Status)
		change.Status = &status
	}

	return change
}

func (r problemResponse) VisitApplyWorkspaceIssueBulkChangeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceBulkActionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

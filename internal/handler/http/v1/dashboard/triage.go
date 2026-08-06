package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceTriage(
	ctx context.Context,
	request api.ListWorkspaceTriageRequestObject,
) (api.ListWorkspaceTriageResponseObject, error) {
	input := service.TriageQueueInput{}

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	queue, err := h.triages.Queue(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceTriage200JSONResponse(triageQueueDTO(queue)), nil
}

func (h *handler) AcceptWorkspaceTriageIssue(
	ctx context.Context,
	request api.AcceptWorkspaceTriageIssueRequestObject,
) (api.AcceptWorkspaceTriageIssueResponseObject, error) {
	issue, err := h.triages.Accept(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AcceptWorkspaceTriageIssue200JSONResponse(issueDTO(issue)), nil
}

func (h *handler) DeclineWorkspaceTriageIssue(
	ctx context.Context,
	request api.DeclineWorkspaceTriageIssueRequestObject,
) (api.DeclineWorkspaceTriageIssueResponseObject, error) {
	issue, err := h.triages.Decline(ctx, request.WorkspaceId, request.IssueId, service.DeclineTriageInput{
		Reason: entity.TriageDeclineReason(request.Body.Reason),
		Note:   noteOf(request.Body.Note),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeclineWorkspaceTriageIssue200JSONResponse(issueDTO(issue)), nil
}

func (h *handler) MergeWorkspaceTriageIssue(
	ctx context.Context,
	request api.MergeWorkspaceTriageIssueRequestObject,
) (api.MergeWorkspaceTriageIssueResponseObject, error) {
	issue, err := h.triages.Merge(
		ctx, request.WorkspaceId, request.IssueId, request.Body.DuplicateOfId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.MergeWorkspaceTriageIssue200JSONResponse(issueDTO(issue)), nil
}

func (h *handler) ReassignWorkspaceTriageIssue(
	ctx context.Context,
	request api.ReassignWorkspaceTriageIssueRequestObject,
) (api.ReassignWorkspaceTriageIssueResponseObject, error) {
	input := service.ReassignTriageInput{
		TeamID:          request.Body.TeamId,
		ExpectedVersion: int(request.Body.ExpectedVersion),
		AcknowledgeLabelLoss: request.Body.AcknowledgeLabelLoss != nil &&
			*request.Body.AcknowledgeLabelLoss,
	}

	issue, err := h.triages.Reassign(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReassignWorkspaceTriageIssue200JSONResponse(issueDTO(issue)), nil
}

func (h *handler) GetTeamTriageSettings(
	ctx context.Context,
	request api.GetTeamTriageSettingsRequestObject,
) (api.GetTeamTriageSettingsResponseObject, error) {
	settings, err := h.triages.Settings(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetTeamTriageSettings200JSONResponse(triageSettingsDTO(settings)), nil
}

func (h *handler) SetTeamTriageSettings(
	ctx context.Context,
	request api.SetTeamTriageSettingsRequestObject,
) (api.SetTeamTriageSettingsResponseObject, error) {
	settings, err := h.triages.Configure(
		ctx, request.WorkspaceId, request.TeamId,
		service.ConfigureTriageInput{
			RouteAgents:       request.Body.RouteAgents,
			RouteIntegrations: request.Body.RouteIntegrations,
			RouteNonMembers:   request.Body.RouteNonMembers,
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetTeamTriageSettings200JSONResponse(triageSettingsDTO(settings)), nil
}

func (h *handler) DeleteTeamTriageSettings(
	ctx context.Context,
	request api.DeleteTeamTriageSettingsRequestObject,
) (api.DeleteTeamTriageSettingsResponseObject, error) {
	if err := h.triages.Disable(ctx, request.WorkspaceId, request.TeamId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteTeamTriageSettings204Response{}, nil
}

func noteOf(note *string) string {
	if note == nil {
		return ""
	}

	return *note
}

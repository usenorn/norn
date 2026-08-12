package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceAgents(
	ctx context.Context,
	request api.ListWorkspaceAgentsRequestObject,
) (api.ListWorkspaceAgentsResponseObject, error) {
	agents, err := h.agents.List(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceAgents200JSONResponse(workspaceAgentDTOs(agents)), nil
}

func (h *handler) RegisterWorkspaceAgent(
	ctx context.Context,
	request api.RegisterWorkspaceAgentRequestObject,
) (api.RegisterWorkspaceAgentResponseObject, error) {
	input := service.RegisterAgentInput{
		WorkspaceID: request.WorkspaceId,
		Name:        request.Body.Name,
		Scopes:      entity.NewAPIScopeSet(request.Body.Scopes),
		AllTeams:    request.Body.AllTeams,
	}

	if request.Body.OwnerAccountId != nil {
		input.OwnerAccountID = *request.Body.OwnerAccountId
	}

	if request.Body.TeamIds != nil {
		input.TeamIDs = append(input.TeamIDs, *request.Body.TeamIds...)
	}

	if request.Body.ActionLimit != nil {
		limit := int(*request.Body.ActionLimit)
		input.ActionLimit = &limit
	}

	registered, err := h.agents.Register(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RegisterWorkspaceAgent201JSONResponse{
		Agent: agentDTO(registered.Agent),
		Value: registered.Value,
	}, nil
}

func (h *handler) GetWorkspaceAgent(
	ctx context.Context,
	request api.GetWorkspaceAgentRequestObject,
) (api.GetWorkspaceAgentResponseObject, error) {
	agent, err := h.agents.Get(ctx, request.WorkspaceId, request.AgentId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceAgent200JSONResponse(workspaceAgentDTO(agent)), nil
}

func (h *handler) DisableWorkspaceAgent(
	ctx context.Context,
	request api.DisableWorkspaceAgentRequestObject,
) (api.DisableWorkspaceAgentResponseObject, error) {
	if err := h.agents.Disable(ctx, request.WorkspaceId, request.AgentId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DisableWorkspaceAgent204Response{}, nil
}

func (h *handler) RotateWorkspaceAgentCredential(
	ctx context.Context,
	request api.RotateWorkspaceAgentCredentialRequestObject,
) (api.RotateWorkspaceAgentCredentialResponseObject, error) {
	rotated, err := h.agents.Rotate(ctx, request.WorkspaceId, request.AgentId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RotateWorkspaceAgentCredential201JSONResponse{
		Agent: agentDTO(rotated.Agent),
		Value: rotated.Value,
	}, nil
}

func (h *handler) ListWorkspaceAgentActivity(
	ctx context.Context,
	request api.ListWorkspaceAgentActivityRequestObject,
) (api.ListWorkspaceAgentActivityResponseObject, error) {
	page := entity.ActivityPage{Order: entity.ActivityOrderNewest}

	if request.Params.Limit != nil {
		page.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil && *request.Params.Cursor != "" {
		cursor, err := entity.DecodeActivityCursor(*request.Params.Cursor)
		if err != nil {
			if problem, ok := problemFor(err); ok {
				return problem, nil
			}

			return nil, err
		}

		page.Cursor = &cursor
	}

	activity, err := h.agents.Activity(ctx, request.WorkspaceId, request.AgentId, page)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceAgentActivity200JSONResponse(activityPageDTO(activity)), nil
}

func (h *handler) GetTeamAgentSettings(
	ctx context.Context,
	request api.GetTeamAgentSettingsRequestObject,
) (api.GetTeamAgentSettingsResponseObject, error) {
	settings, err := h.agents.Settings(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetTeamAgentSettings200JSONResponse(agentSettingsDTO(settings)), nil
}

func (h *handler) SetTeamAgentSettings(
	ctx context.Context,
	request api.SetTeamAgentSettingsRequestObject,
) (api.SetTeamAgentSettingsResponseObject, error) {
	settings, err := h.agents.Configure(ctx, service.ConfigureAgentInput{
		WorkspaceID:       request.WorkspaceId,
		TeamID:            request.TeamId,
		HoldComments:      entity.AgentHold(request.Body.HoldComments),
		HoldStateChanges:  entity.AgentHold(request.Body.HoldStateChanges),
		HoldIssueEdits:    entity.AgentHold(request.Body.HoldIssueEdits),
		HoldIssueCreation: entity.AgentHold(request.Body.HoldIssueCreation),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetTeamAgentSettings200JSONResponse(agentSettingsDTO(settings)), nil
}

func (h *handler) ListWorkspaceAgentProposals(
	ctx context.Context,
	request api.ListWorkspaceAgentProposalsRequestObject,
) (api.ListWorkspaceAgentProposalsResponseObject, error) {
	waiting, err := h.agents.Waiting(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceAgentProposals200JSONResponse(waitingProposalDTOs(waiting)), nil
}

func (h *handler) ApproveWorkspaceAgentProposal(
	ctx context.Context,
	request api.ApproveWorkspaceAgentProposalRequestObject,
) (api.ApproveWorkspaceAgentProposalResponseObject, error) {
	input := service.ApproveProposalInput{}

	if request.Body != nil {
		input.Edited = true
		input.Checks = make([]service.ProposedCheckEdit, 0, len(request.Body.Checks))

		for _, edit := range request.Body.Checks {
			edited := service.ProposedCheckEdit{
				Statement: edit.Statement,
				Method:    entity.CheckMethod(edit.Method),
				Proof:     edit.Proof,
				TimeLimit: checkTimeLimit(edit.TimeLimitSeconds),
			}

			if edit.Id != nil {
				edited.CheckID = *edit.Id
			}

			input.Checks = append(input.Checks, edited)
		}
	}

	proposal, err := h.agents.Approve(ctx, request.WorkspaceId, request.ProposalId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ApproveWorkspaceAgentProposal200JSONResponse(agentProposalDTO(proposal)), nil
}

func (h *handler) RejectWorkspaceAgentProposal(
	ctx context.Context,
	request api.RejectWorkspaceAgentProposalRequestObject,
) (api.RejectWorkspaceAgentProposalResponseObject, error) {
	proposal, err := h.agents.Reject(ctx, request.WorkspaceId, request.ProposalId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RejectWorkspaceAgentProposal200JSONResponse(agentProposalDTO(proposal)), nil
}

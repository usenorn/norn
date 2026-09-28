package dashboard

import (
	"context"
	"errors"

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

func (h *handler) ListGrantableAgentScopes(
	ctx context.Context,
	request api.ListGrantableAgentScopesRequestObject,
) (api.ListGrantableAgentScopesResponseObject, error) {
	scopes, err := h.agents.GrantableScopes(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListGrantableAgentScopes200JSONResponse{Scopes: apiScopeDTOs(scopes)}, nil
}

func (h *handler) RegisterWorkspaceAgent(
	ctx context.Context,
	request api.RegisterWorkspaceAgentRequestObject,
) (api.RegisterWorkspaceAgentResponseObject, error) {
	input := service.RegisterAgentInput{
		WorkspaceID: request.WorkspaceId,
		Name:        request.Body.Name,
		Scopes:      apiScopes(request.Body.Scopes),
		AllTeams:    request.Body.AllTeams,
	}

	if request.Body.Icon != nil {
		input.Icon = entity.AgentIcon(*request.Body.Icon)
	}

	if request.Body.Scope != nil {
		input.Scope = entity.AgentScope(*request.Body.Scope)
	}

	input.ProjectID = request.Body.ProjectId

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

func (h *handler) SetWorkspaceAgentInstructions(
	ctx context.Context,
	request api.SetWorkspaceAgentInstructionsRequestObject,
) (api.SetWorkspaceAgentInstructionsResponseObject, error) {
	agent, err := h.agents.SetInstructions(ctx, service.SetAgentInstructionsInput{
		WorkspaceID:  request.WorkspaceId,
		AgentID:      request.AgentId,
		Instructions: request.Body.Instructions,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceAgentInstructions200JSONResponse(workspaceAgentDTO(agent)), nil
}

func (h *handler) SetWorkspaceAgentScope(
	ctx context.Context,
	request api.SetWorkspaceAgentScopeRequestObject,
) (api.SetWorkspaceAgentScopeResponseObject, error) {
	agent, err := h.agents.Rescope(ctx, service.RescopeAgentInput{
		WorkspaceID: request.WorkspaceId,
		AgentID:     request.AgentId,
		Scope:       entity.AgentScope(request.Body.Scope),
		ProjectID:   request.Body.ProjectId,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceAgentScope200JSONResponse(workspaceAgentDTO(agent)), nil
}

func (h *handler) SetWorkspaceAgentExecution(
	ctx context.Context,
	request api.SetWorkspaceAgentExecutionRequestObject,
) (api.SetWorkspaceAgentExecutionResponseObject, error) {
	agent, err := h.agents.SetExecution(ctx, service.SetAgentExecutionInput{
		WorkspaceID: request.WorkspaceId,
		AgentID:     request.AgentId,
		Execution:   entity.AgentExecution(request.Body.Execution),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetWorkspaceAgentExecution200JSONResponse(workspaceAgentDTO(agent)), nil
}

func (h *handler) ConverseWithWorkspaceAgent(
	ctx context.Context,
	request api.ConverseWithWorkspaceAgentRequestObject,
) (api.ConverseWithWorkspaceAgentResponseObject, error) {
	turns := make([]entity.AgentTurn, 0, len(request.Body.Turns))

	for _, turn := range request.Body.Turns {
		turns = append(turns, entity.AgentTurn{Role: entity.AgentTurnRole(turn.Role), Text: turn.Text})
	}

	reply, err := h.hostedAgents.Converse(ctx, request.WorkspaceId, request.AgentId, turns)
	if err != nil {
		if errors.Is(err, entity.ErrAIProviderNotConfigured) {
			return agentUnusableProblem(api.AgentUnusableProblemCodeAiProviderNotConfigured, err), nil
		}

		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConverseWithWorkspaceAgent200JSONResponse(agentReplyDTO(reply)), nil
}

func agentReplyDTO(reply entity.AgentReply) api.AgentConversationReply {
	calls := make([]api.AgentToolCall, 0, len(reply.ToolCalls))

	for _, call := range reply.ToolCalls {
		calls = append(calls, api.AgentToolCall{Name: call.Name, Refusal: nilIfEmpty(call.Refusal)})
	}

	return api.AgentConversationReply{
		Text:      reply.Text,
		ToolCalls: calls,
		Usage: api.AgentConversationUsage{
			InputTokens:  int32(reply.Usage.Input),
			OutputTokens: int32(reply.Usage.Output),
		},
		Stop: api.AgentConversationStop(reply.Stop),
	}
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

func (h *handler) EnableWorkspaceAgent(
	ctx context.Context,
	request api.EnableWorkspaceAgentRequestObject,
) (api.EnableWorkspaceAgentResponseObject, error) {
	enabled, err := h.agents.Enable(ctx, request.WorkspaceId, request.AgentId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.EnableWorkspaceAgent201JSONResponse{
		Agent: agentDTO(enabled.Agent),
		Value: enabled.Value,
	}, nil
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
	var accepted []entity.ChangePart

	if request.Body != nil && request.Body.Accept != nil {
		for _, part := range *request.Body.Accept {
			accepted = append(accepted, entity.ChangePart(part))
		}
	}

	proposal, err := h.agents.Approve(ctx, request.WorkspaceId, request.ProposalId, accepted)
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

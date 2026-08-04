package dashboard

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceCycles(
	ctx context.Context,
	request api.ListWorkspaceCyclesRequestObject,
) (api.ListWorkspaceCyclesResponseObject, error) {
	input := service.ListCyclesInput{TeamID: request.Params.TeamId}

	if request.Params.Phase != nil {
		input.Phase = entity.CyclePhase(*request.Params.Phase)
	}

	views, err := h.cycles.List(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceCycles200JSONResponse(cycleDTOs(views)), nil
}

func (h *handler) ListCurrentWorkspaceCycles(
	ctx context.Context,
	request api.ListCurrentWorkspaceCyclesRequestObject,
) (api.ListCurrentWorkspaceCyclesResponseObject, error) {
	current, err := h.cycles.Current(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dtos := make([]api.TeamCycle, 0, len(current))

	for _, entry := range current {
		dtos = append(dtos, api.TeamCycle{TeamId: entry.TeamID, Cycle: cycleDTO(entry.View)})
	}

	return api.ListCurrentWorkspaceCycles200JSONResponse(dtos), nil
}

func (h *handler) GetWorkspaceCycle(
	ctx context.Context,
	request api.GetWorkspaceCycleRequestObject,
) (api.GetWorkspaceCycleResponseObject, error) {
	view, err := h.cycles.Get(ctx, request.WorkspaceId, request.CycleId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceCycle200JSONResponse(cycleDTO(view)), nil
}

func (h *handler) GetWorkspaceCycleScope(
	ctx context.Context,
	request api.GetWorkspaceCycleScopeRequestObject,
) (api.GetWorkspaceCycleScopeResponseObject, error) {
	scope, err := h.cycles.Scope(ctx, request.WorkspaceId, request.CycleId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceCycleScope200JSONResponse(cycleScopeDTO(scope)), nil
}

func (h *handler) CloseWorkspaceCycle(
	ctx context.Context,
	request api.CloseWorkspaceCycleRequestObject,
) (api.CloseWorkspaceCycleResponseObject, error) {
	input := service.CloseCycleInput{}

	if request.Body.Rollover != nil {
		input.Rollover = entity.CycleRollover(*request.Body.Rollover)
	}

	if request.Body.Overrides != nil {
		for _, override := range *request.Body.Overrides {
			input.Overrides = append(input.Overrides, service.RolloverOverride{
				IssueID:     override.IssueId,
				Destination: entity.CycleRollover(override.Destination),
			})
		}
	}

	view, err := h.cycles.Close(ctx, request.WorkspaceId, request.CycleId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CloseWorkspaceCycle200JSONResponse(cycleDTO(view)), nil
}

func (h *handler) GetTeamCycleCadence(
	ctx context.Context,
	request api.GetTeamCycleCadenceRequestObject,
) (api.GetTeamCycleCadenceResponseObject, error) {
	view, err := h.cycles.Cadence(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetTeamCycleCadence200JSONResponse(cadenceDTO(view)), nil
}

func (h *handler) SetTeamCycleCadence(
	ctx context.Context,
	request api.SetTeamCycleCadenceRequestObject,
) (api.SetTeamCycleCadenceResponseObject, error) {
	view, err := h.cycles.SetCadence(ctx, request.WorkspaceId, request.TeamId, service.SetCadenceInput{
		LengthWeeks: int(request.Body.LengthWeeks),
		StartsOn:    time.Weekday(request.Body.StartsOn),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetTeamCycleCadence200JSONResponse(cadenceDTO(view)), nil
}

func (h *handler) DeleteTeamCycleCadence(
	ctx context.Context,
	request api.DeleteTeamCycleCadenceRequestObject,
) (api.DeleteTeamCycleCadenceResponseObject, error) {
	if err := h.cycles.DisableCadence(ctx, request.WorkspaceId, request.TeamId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteTeamCycleCadence204Response{}, nil
}

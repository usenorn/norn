package mcpserver

import (
	"context"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type listCyclesInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Team      string `json:"team,omitempty" jsonschema:"a team key like ENG, or a team id"`
	Phase     string `json:"phase,omitempty" jsonschema:"upcoming, current, ended, or closed"`
}

type listCyclesOutput struct {
	Cycles []cycleDTO `json:"cycles"`
}

func (t *toolset) listCycles(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input listCyclesInput,
) (*mcp.CallToolResult, listCyclesOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, listCyclesOutput{}, toolFailure(ctx, err)
	}

	list := service.ListCyclesInput{Phase: entity.CyclePhase(input.Phase)}

	if input.Team != "" {
		team, err := t.resolveTeam(ctx, workspace.ID, input.Team)
		if err != nil {
			return nil, listCyclesOutput{}, toolFailure(ctx, err)
		}

		teamID := team.ID
		list.TeamID = &teamID
	}

	views, err := t.cycles.List(ctx, workspace.ID, list)
	if err != nil {
		return nil, listCyclesOutput{}, toolFailure(ctx, err)
	}

	output := listCyclesOutput{Cycles: make([]cycleDTO, 0, len(views))}

	for _, view := range views {
		output.Cycles = append(output.Cycles, cycleDTOFrom(view))
	}

	return nil, output, nil
}

type getCycleInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Cycle     string `json:"cycle" jsonschema:"the cycle id from norn_list_cycles"`
}

type cycleScopeChangeDTO struct {
	IssueReference string `json:"issueReference"`
	IssueTitle     string `json:"issueTitle"`
	Change         string `json:"change"`
}

type getCycleOutput struct {
	Cycle    cycleDTO              `json:"cycle"`
	Original []issueDTO            `json:"original"`
	Added    []issueDTO            `json:"added"`
	Changes  []cycleScopeChangeDTO `json:"changes,omitempty"`
}

func (t *toolset) getCycle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getCycleInput,
) (*mcp.CallToolResult, getCycleOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, getCycleOutput{}, toolFailure(ctx, err)
	}

	cycleID, err := uuid.Parse(input.Cycle)
	if err != nil {
		return nil, getCycleOutput{}, toolFailure(ctx, entity.ErrCycleNotFound)
	}

	view, err := t.cycles.Get(ctx, workspace.ID, cycleID)
	if err != nil {
		return nil, getCycleOutput{}, toolFailure(ctx, err)
	}

	scope, err := t.cycles.Scope(ctx, workspace.ID, cycleID)
	if err != nil {
		return nil, getCycleOutput{}, toolFailure(ctx, err)
	}

	output := getCycleOutput{
		Cycle:    cycleDTOFrom(view),
		Original: issueDTOsFrom(scope.Original),
		Added:    issueDTOsFrom(scope.Added),
	}

	for _, change := range scope.Changes {
		output.Changes = append(output.Changes, cycleScopeChangeDTO{
			IssueReference: change.IssueReference,
			IssueTitle:     change.IssueTitle,
			Change:         string(change.Change),
		})
	}

	return nil, output, nil
}

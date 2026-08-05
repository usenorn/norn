package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

type listWorkspacesOutput struct {
	Workspaces []workspaceDTO `json:"workspaces"`
}

func (t *toolset) listWorkspaces(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ struct{},
) (*mcp.CallToolResult, listWorkspacesOutput, error) {
	actor, ok := identity.Actor(ctx)
	if !ok {
		return nil, listWorkspacesOutput{}, toolFailure(ctx, entity.ErrAccountForbidden)
	}

	reachable, err := t.workspaces.ListForAccount(ctx, actor.AccountID)
	if err != nil {
		return nil, listWorkspacesOutput{}, toolFailure(ctx, err)
	}

	output := listWorkspacesOutput{Workspaces: make([]workspaceDTO, 0, len(reachable))}

	for _, workspace := range reachable {
		if !actor.ConfinedTo(workspace.ID) {
			continue
		}

		output.Workspaces = append(output.Workspaces, workspaceDTOFrom(workspace))
	}

	return nil, output, nil
}

type workspaceStructureInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
}

type workspaceStructureOutput struct {
	Teams  []teamDTO  `json:"teams"`
	Labels []labelDTO `json:"labels"`
}

func (t *toolset) getWorkspaceStructure(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input workspaceStructureInput,
) (*mcp.CallToolResult, workspaceStructureOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, workspaceStructureOutput{}, toolFailure(ctx, err)
	}

	teams, err := t.teams.List(ctx, workspace.ID, entity.TeamStatusActive)
	if err != nil {
		return nil, workspaceStructureOutput{}, toolFailure(ctx, err)
	}

	output := workspaceStructureOutput{Teams: make([]teamDTO, 0, len(teams))}

	for _, team := range teams {
		dto := teamDTO{
			ID:         team.ID.String(),
			Key:        team.Key,
			Name:       team.Name,
			Visibility: string(team.Visibility),
		}

		states, err := t.workflowStates.List(ctx, workspace.ID, team.ID)
		if err != nil {
			return nil, workspaceStructureOutput{}, toolFailure(ctx, err)
		}

		for _, state := range states {
			dto.States = append(dto.States, stateDTOFrom(state))
		}

		output.Teams = append(output.Teams, dto)
	}

	labels, err := t.labels.List(ctx, workspace.ID)
	if err != nil {
		return nil, workspaceStructureOutput{}, toolFailure(ctx, err)
	}

	output.Labels = make([]labelDTO, 0, len(labels))

	for _, label := range labels {
		output.Labels = append(output.Labels, labelDTOFrom(label))
	}

	return nil, output, nil
}

type listMembersInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Query     string `json:"query,omitempty" jsonschema:"filter members by name or email"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"the cursor returned by the previous page"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size"`
}

type listMembersOutput struct {
	Members    []memberDTO `json:"members"`
	NextCursor string      `json:"nextCursor,omitempty"`
}

func (t *toolset) listWorkspaceMembers(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input listMembersInput,
) (*mcp.CallToolResult, listMembersOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, listMembersOutput{}, toolFailure(ctx, err)
	}

	page, err := t.workspaces.ListMembers(ctx, workspace.ID, service.ListMembersInput{
		Query:  input.Query,
		Cursor: input.Cursor,
		Limit:  input.Limit,
	})
	if err != nil {
		return nil, listMembersOutput{}, toolFailure(ctx, err)
	}

	output := listMembersOutput{
		Members:    make([]memberDTO, 0, len(page.Members)),
		NextCursor: page.NextCursor,
	}

	for _, member := range page.Members {
		output.Members = append(output.Members, memberDTOFrom(member))
	}

	return nil, output, nil
}

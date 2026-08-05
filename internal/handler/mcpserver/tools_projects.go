package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type listProjectsInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	State     string `json:"state,omitempty" jsonschema:"planned, active, paused, completed, or cancelled"`
	Archived  bool   `json:"archived,omitempty" jsonschema:"include archived projects only"`
	Mine      bool   `json:"mine,omitempty" jsonschema:"only projects led by or including the authorizing person"`
}

type listProjectsOutput struct {
	Projects []projectDTO `json:"projects"`
}

func (t *toolset) listProjects(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input listProjectsInput,
) (*mcp.CallToolResult, listProjectsOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, listProjectsOutput{}, toolFailure(ctx, err)
	}

	views, err := t.projects.List(ctx, workspace.ID, service.ListProjectsInput{
		State:    entity.ProjectState(input.State),
		Archived: input.Archived,
		Mine:     input.Mine,
	})
	if err != nil {
		return nil, listProjectsOutput{}, toolFailure(ctx, err)
	}

	output := listProjectsOutput{Projects: make([]projectDTO, 0, len(views))}

	for _, view := range views {
		output.Projects = append(output.Projects, projectDTOFrom(view))
	}

	return nil, output, nil
}

type getProjectInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Project   string `json:"project" jsonschema:"the project slug or id"`
}

type getProjectOutput struct {
	Project       projectDTO         `json:"project"`
	StatusUpdates []projectStatusDTO `json:"statusUpdates,omitempty"`
}

func (t *toolset) getProject(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getProjectInput,
) (*mcp.CallToolResult, getProjectOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	view, err := t.resolveProject(ctx, workspace.ID, input.Project)
	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	updates, err := t.projects.ListStatus(ctx, workspace.ID, view.Project.ID)
	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	output := getProjectOutput{Project: projectDTOFrom(view)}

	for _, update := range updates {
		output.StatusUpdates = append(output.StatusUpdates, projectStatusDTOFrom(update))
	}

	return nil, output, nil
}

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
	Team      string `json:"team,omitempty" jsonschema:"only projects serving this team key or id"`
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

	list := service.ListProjectsInput{
		State:    entity.ProjectState(input.State),
		Archived: input.Archived,
		Mine:     input.Mine,
	}

	if input.Team != "" {
		team, err := t.resolveTeam(ctx, workspace.ID, input.Team)
		if err != nil {
			return nil, listProjectsOutput{}, toolFailure(ctx, err)
		}

		list.TeamID = &team.ID
	}

	views, err := t.projects.List(ctx, workspace.ID, list)
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

type createProjectInput struct {
	Workspace   string   `json:"workspace" jsonschema:"the workspace slug or id"`
	Slug        string   `json:"slug" jsonschema:"the project slug, lowercase and hyphenated"`
	Name        string   `json:"name" jsonschema:"the project name"`
	Description string   `json:"description,omitempty" jsonschema:"what the project is for, markdown"`
	Lead        string   `json:"lead,omitempty" jsonschema:"an account id, or me for the authorizing person"`
	TargetOn    string   `json:"target_on,omitempty" jsonschema:"the target date as YYYY-MM-DD"`
	Teams       []string `json:"teams,omitempty" jsonschema:"team keys or ids this project serves; none means the whole workspace"`
}

func (t *toolset) createProject(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input createProjectInput,
) (*mcp.CallToolResult, getProjectOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	create := service.CreateProjectInput{
		WorkspaceID: workspace.ID,
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
		TargetOn:    input.TargetOn,
	}

	if input.Lead != "" {
		accountID, err := resolveAssignee(ctx, input.Lead)
		if err != nil {
			return nil, getProjectOutput{}, toolFailure(ctx, err)
		}

		create.LeadAccountID = &accountID
	}

	if create.TeamIDs, err = t.resolveTeams(ctx, workspace.ID, input.Teams); err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	view, err := t.projects.Create(ctx, create)
	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	return nil, getProjectOutput{Project: projectDTOFrom(view)}, nil
}

type updateProjectInput struct {
	Workspace   string    `json:"workspace" jsonschema:"the workspace slug or id"`
	Project     string    `json:"project" jsonschema:"the project slug or id"`
	Name        *string   `json:"name,omitempty" jsonschema:"a new name"`
	Description *string   `json:"description,omitempty" jsonschema:"a new description, markdown"`
	State       *string   `json:"state,omitempty" jsonschema:"planned, active, paused, completed, or cancelled"`
	Lead        *string   `json:"lead,omitempty" jsonschema:"an account id, or me for the authorizing person"`
	TargetOn    *string   `json:"target_on,omitempty" jsonschema:"a new target date as YYYY-MM-DD"`
	Teams       *[]string `json:"teams,omitempty" jsonschema:"replace the teams this project serves; an empty list makes it the whole workspace's"`
	Clear       []string  `json:"clear,omitempty" jsonschema:"fields to clear: lead or targetOn"`
}

func (t *toolset) updateProject(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input updateProjectInput,
) (*mcp.CallToolResult, getProjectOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	project, err := t.resolveProject(ctx, workspace.ID, input.Project)
	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	view := project

	if input.State != nil {
		view, err = t.projects.SetState(
			ctx, workspace.ID, project.Project.ID, entity.ProjectState(*input.State),
		)
		if err != nil {
			return nil, getProjectOutput{}, toolFailure(ctx, err)
		}
	}

	update := service.UpdateProjectInput{
		Name:        input.Name,
		Description: input.Description,
		TargetOn:    input.TargetOn,
		Clear:       input.Clear,
	}

	if input.Lead != nil {
		accountID, err := resolveAssignee(ctx, *input.Lead)
		if err != nil {
			return nil, getProjectOutput{}, toolFailure(ctx, err)
		}

		update.LeadAccountID = &accountID
	}

	if input.Teams != nil {
		teamIDs, err := t.resolveTeams(ctx, workspace.ID, *input.Teams)
		if err != nil {
			return nil, getProjectOutput{}, toolFailure(ctx, err)
		}

		update.TeamIDs = &teamIDs
	}

	if changesProjectSettings(input) {
		view, err = t.projects.Update(ctx, workspace.ID, project.Project.ID, update)
		if err != nil {
			return nil, getProjectOutput{}, toolFailure(ctx, err)
		}
	}

	return nil, getProjectOutput{Project: projectDTOFrom(view)}, nil
}

func changesProjectSettings(input updateProjectInput) bool {
	return input.Name != nil ||
		input.Description != nil ||
		input.Lead != nil ||
		input.TargetOn != nil ||
		input.Teams != nil ||
		len(input.Clear) > 0
}

type archiveProjectInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Project   string `json:"project" jsonschema:"the project slug or id"`
	Archived  bool   `json:"archived" jsonschema:"true to archive, false to bring it back"`
}

func (t *toolset) archiveProject(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input archiveProjectInput,
) (*mcp.CallToolResult, getProjectOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	project, err := t.resolveProject(ctx, workspace.ID, input.Project)
	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	var view service.ProjectView

	if input.Archived {
		view, err = t.projects.Archive(ctx, workspace.ID, project.Project.ID)
	} else {
		view, err = t.projects.Unarchive(ctx, workspace.ID, project.Project.ID)
	}

	if err != nil {
		return nil, getProjectOutput{}, toolFailure(ctx, err)
	}

	return nil, getProjectOutput{Project: projectDTOFrom(view)}, nil
}

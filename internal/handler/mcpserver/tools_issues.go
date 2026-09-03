package mcpserver

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type getIssueInput struct {
	Workspace       string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue           string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	IncludeComments bool   `json:"include_comments,omitempty" jsonschema:"also return the comment thread"`
}

type getIssueOutput struct {
	Issue     issueDTO      `json:"issue"`
	Comments  []commentDTO  `json:"comments,omitempty"`
	Questions []questionDTO `json:"questions,omitempty"`
}

func (t *toolset) getIssue(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getIssueInput,
) (*mcp.CallToolResult, getIssueOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, getIssueOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, getIssueOutput{}, toolFailure(ctx, err)
	}

	output := getIssueOutput{Issue: issueDTOFrom(issue)}

	if input.IncludeComments {
		thread, err := t.issueComments.List(ctx, workspace.ID, issue.ID, service.ListCommentsInput{})
		if err != nil {
			return nil, getIssueOutput{}, toolFailure(ctx, err)
		}

		for _, comment := range thread.Comments {
			output.Comments = append(output.Comments, commentDTOFrom(comment))
		}
	}

	asked, err := t.questions.List(ctx, workspace.ID, issue.ID)
	if err != nil {
		return nil, getIssueOutput{}, toolFailure(ctx, err)
	}

	output.Questions = questionDTOs(entity.UnansweredQuestions(asked))

	return nil, output, nil
}

type listIssuesInput struct {
	Workspace     string `json:"workspace" jsonschema:"the workspace slug or id"`
	Team          string `json:"team,omitempty" jsonschema:"a team key like ENG, or a team id"`
	State         string `json:"state,omitempty" jsonschema:"a workflow state name or id; a name needs team set as well"`
	StateCategory string `json:"state_category,omitempty" jsonschema:"not_started, active, complete, or abandoned"`
	Assignee      string `json:"assignee,omitempty" jsonschema:"an account id, or me for the authorizing person"`
	Priority      string `json:"priority,omitempty" jsonschema:"urgent, high, medium, low, or none"`
	Project       string `json:"project,omitempty" jsonschema:"a project slug or id"`
	Cycle         string `json:"cycle,omitempty" jsonschema:"a cycle id"`
	Text          string `json:"text,omitempty" jsonschema:"full-text query over titles and descriptions"`
	Cursor        string `json:"cursor,omitempty" jsonschema:"the cursor returned by the previous page"`
	Limit         int    `json:"limit,omitempty" jsonschema:"page size, at most 200"`
}

type listIssuesOutput struct {
	Issues     []issueDTO `json:"issues"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

func (t *toolset) listIssues(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input listIssuesInput,
) (*mcp.CallToolResult, listIssuesOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, listIssuesOutput{}, toolFailure(ctx, err)
	}

	filter, err := t.compileFilter(ctx, workspace.ID, input)
	if err != nil {
		return nil, listIssuesOutput{}, toolFailure(ctx, err)
	}

	result, err := t.issues.Query(ctx, workspace.ID, service.QueryIssuesInput{
		Text:   input.Text,
		Filter: filter,
		Cursor: input.Cursor,
		Limit:  input.Limit,
	})
	if err != nil {
		return nil, listIssuesOutput{}, toolFailure(ctx, err)
	}

	return nil, listIssuesOutput{
		Issues:     issueDTOsFrom(result.Issues),
		NextCursor: result.NextCursor,
	}, nil
}

func (t *toolset) compileFilter(
	ctx context.Context,
	workspaceID uuid.UUID,
	input listIssuesInput,
) (*entity.IssueFilter, error) {
	leaves := make([]entity.IssueFilter, 0, 6)

	leaf := func(field entity.IssueFilterField, value string) {
		leaves = append(leaves, entity.IssueFilter{
			Field:  field,
			Op:     entity.IssueFilterOpIs,
			Values: []string{value},
		})
	}

	var team entity.Team

	if input.Team != "" {
		resolved, err := t.resolveTeam(ctx, workspaceID, input.Team)
		if err != nil {
			return nil, err
		}

		team = resolved

		leaf(entity.IssueFilterFieldTeam, team.ID.String())
	}

	if input.State != "" {
		if team.ID == uuid.Nil {
			return nil, errors.New("filtering by state name needs the team parameter as well")
		}

		state, err := t.resolveState(ctx, workspaceID, team.ID, input.State)
		if err != nil {
			return nil, err
		}

		leaf(entity.IssueFilterFieldState, state.ID.String())
	}

	if input.StateCategory != "" {
		leaf(entity.IssueFilterFieldStateCategory, input.StateCategory)
	}

	if input.Assignee != "" {
		accountID, err := resolveAssignee(ctx, input.Assignee)
		if err != nil {
			return nil, err
		}

		leaf(entity.IssueFilterFieldAssignee, accountID.String())
	}

	if input.Priority != "" {
		leaf(entity.IssueFilterFieldPriority, input.Priority)
	}

	if input.Project != "" {
		project, err := t.resolveProject(ctx, workspaceID, input.Project)
		if err != nil {
			return nil, err
		}

		leaf(entity.IssueFilterFieldProject, project.Project.ID.String())
	}

	if input.Cycle != "" {
		cycleID, err := uuid.Parse(input.Cycle)
		if err != nil {
			return nil, errors.New("cycle must be a cycle id from norn_list_cycles")
		}

		leaf(entity.IssueFilterFieldCycle, cycleID.String())
	}

	if len(leaves) == 0 {
		return nil, nil
	}

	return &entity.IssueFilter{All: leaves}, nil
}

type createIssueInput struct {
	Workspace   string   `json:"workspace" jsonschema:"the workspace slug or id"`
	Team        string   `json:"team" jsonschema:"the team key like ENG, or a team id"`
	Title       string   `json:"title" jsonschema:"the issue title"`
	Description string   `json:"description,omitempty" jsonschema:"the issue description, markdown"`
	Priority    string   `json:"priority,omitempty" jsonschema:"urgent, high, medium, low, or none"`
	Assignee    string   `json:"assignee,omitempty" jsonschema:"an account id, or me for the authorizing person"`
	Project     string   `json:"project,omitempty" jsonschema:"a project slug or id to raise the issue in"`
	Labels      []string `json:"labels,omitempty" jsonschema:"label names or ids to put on the issue"`
	Estimate    int      `json:"estimate,omitempty" jsonschema:"the effort estimate in points"`
	DueOn       string   `json:"due_on,omitempty" jsonschema:"the due date as YYYY-MM-DD"`
}

type issueOutput struct {
	Issue issueDTO `json:"issue"`
}

func (t *toolset) createIssue(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input createIssueInput,
) (*mcp.CallToolResult, issueOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	team, err := t.resolveTeam(ctx, workspace.ID, input.Team)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	create := service.CreateIssueInput{
		WorkspaceID: workspace.ID,
		TeamID:      team.ID,
		Title:       input.Title,
		Description: input.Description,
		Priority:    entity.IssuePriority(input.Priority),
		Estimate:    input.Estimate,
		DueOn:       input.DueOn,
	}

	if input.Assignee != "" {
		accountID, err := resolveAssignee(ctx, input.Assignee)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		create.AssigneeAccountID = accountID
	}

	if input.Project != "" {
		project, err := t.resolveProject(ctx, workspace.ID, input.Project)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		create.ProjectID = project.Project.ID
	}

	if len(input.Labels) > 0 {
		labelIDs, err := t.resolveLabels(ctx, workspace.ID, input.Labels)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		create.LabelIDs = labelIDs
	}

	issue, err := t.issues.Create(ctx, create)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	return nil, issueOutput{Issue: issueDTOFrom(issue)}, nil
}

type updateIssueInput struct {
	Workspace          string   `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue              string   `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	ExpectedVersion    int      `json:"expected_version" jsonschema:"the version returned by norn_get_issue"`
	Title              *string  `json:"title,omitempty" jsonschema:"a new title"`
	Description        *string  `json:"description,omitempty" jsonschema:"a new description, markdown"`
	Priority           *string  `json:"priority,omitempty" jsonschema:"urgent, high, medium, low, or none"`
	Assignee           *string  `json:"assignee,omitempty" jsonschema:"an account id, or me for the authorizing person"`
	Estimate           *int     `json:"estimate,omitempty" jsonschema:"a new effort estimate in points"`
	DueOn              *string  `json:"due_on,omitempty" jsonschema:"a new due date as YYYY-MM-DD"`
	State              *string  `json:"state,omitempty" jsonschema:"a workflow state name or id"`
	Project            *string  `json:"project,omitempty" jsonschema:"a project slug or id"`
	Team               *string  `json:"team,omitempty" jsonschema:"move the issue to this team key or id; the issue keeps its reference"`
	DropStrandedLabels *bool    `json:"drop_stranded_labels,omitempty" jsonschema:"allow a move that loses labels the destination team cannot hold"`
	Labels             []string `json:"labels,omitempty" jsonschema:"replace the issue's labels with these names or ids"`
	Clear              []string `json:"clear,omitempty" jsonschema:"fields to clear: assignee, estimate, dueOn, cycle, or project"`
}

func (t *toolset) updateIssue(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input updateIssueInput,
) (*mcp.CallToolResult, issueOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	if input.Team != nil {
		team, err := t.resolveTeam(ctx, workspace.ID, *input.Team)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		move := service.MoveIssueInput{
			ExpectedVersion: input.ExpectedVersion,
			TeamID:          team.ID,
		}

		if input.DropStrandedLabels != nil {
			move.AcknowledgeLabelLoss = *input.DropStrandedLabels
		}

		moved, err := t.issues.MoveToTeam(ctx, workspace.ID, issue.ID, move)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		issue = moved
		input.ExpectedVersion = moved.Version

		if !changesBeyondTeam(input) && input.Labels == nil {
			return nil, issueOutput{Issue: issueDTOFrom(moved)}, nil
		}
	}

	update := service.UpdateIssueInput{
		ExpectedVersion: input.ExpectedVersion,
		Title:           input.Title,
		Description:     input.Description,
		DueOn:           input.DueOn,
		Clear:           input.Clear,
	}

	if input.Priority != nil {
		priority := entity.IssuePriority(*input.Priority)
		update.Priority = &priority
	}

	if input.Estimate != nil {
		update.Estimate = input.Estimate
	}

	if input.Assignee != nil {
		accountID, err := resolveAssignee(ctx, *input.Assignee)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		update.AssigneeID = &accountID
	}

	if input.State != nil {
		state, err := t.resolveState(ctx, workspace.ID, issue.TeamID, *input.State)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		stateID := state.ID
		update.StateID = &stateID
	}

	if input.Project != nil {
		project, err := t.resolveProject(ctx, workspace.ID, *input.Project)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		projectID := project.Project.ID
		update.ProjectID = &projectID
	}

	updated := issue

	if changesBeyondTeam(input) {
		updated, err = t.issues.Update(ctx, workspace.ID, issue.ID, update)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}
	}

	if input.Labels != nil {
		labelIDs, err := t.resolveLabels(ctx, workspace.ID, input.Labels)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		if _, err := t.issues.SetLabels(ctx, workspace.ID, issue.ID, service.SetIssueLabelsInput{
			ExpectedVersion: updated.Version,
			LabelIDs:        labelIDs,
		}); err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		updated, err = t.issues.Get(ctx, workspace.ID, issue.ID)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}
	}

	return nil, issueOutput{Issue: issueDTOFrom(updated)}, nil
}

type changeIssueStateInput struct {
	Workspace       string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue           string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	State           string `json:"state" jsonschema:"the target workflow state name or id"`
	ExpectedVersion int    `json:"expected_version,omitempty" jsonschema:"the version from norn_get_issue; read automatically when omitted"`
}

func (t *toolset) changeIssueState(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input changeIssueStateInput,
) (*mcp.CallToolResult, issueOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	state, err := t.resolveState(ctx, workspace.ID, issue.TeamID, input.State)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	version := input.ExpectedVersion
	if version == 0 {
		version = issue.Version
	}

	stateID := state.ID

	updated, err := t.issues.Update(ctx, workspace.ID, issue.ID, service.UpdateIssueInput{
		ExpectedVersion: version,
		StateID:         &stateID,
	})
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	return nil, issueOutput{Issue: issueDTOFrom(updated)}, nil
}

func changesBeyondTeam(input updateIssueInput) bool {
	return input.Title != nil ||
		input.Description != nil ||
		input.Priority != nil ||
		input.Assignee != nil ||
		input.Estimate != nil ||
		input.DueOn != nil ||
		input.State != nil ||
		input.Project != nil ||
		len(input.Clear) > 0
}

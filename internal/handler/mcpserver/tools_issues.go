package mcpserver

import (
	"context"
	"fmt"
	"slices"
	"strings"

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
	Relations []relationDTO `json:"relations,omitempty"`
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

	related, err := t.relations.List(ctx, workspace.ID, issue.ID)
	if err != nil {
		return nil, getIssueOutput{}, toolFailure(ctx, err)
	}

	output.Relations = relationDTOsFrom(related)

	return nil, output, nil
}

type listIssuesInput struct {
	Workspace     string `json:"workspace" jsonschema:"the workspace slug or id"`
	Team          string `json:"team,omitempty" jsonschema:"a team key like ENG, or a team id"`
	State         string `json:"state,omitempty" jsonschema:"a workflow state name or id; a name needs team set as well"`
	StateCategory string `json:"state_category,omitempty" jsonschema:"not_started, active, complete, or abandoned"`
	Assignee      string `json:"assignee,omitempty" jsonschema:"me for the person this connection acts for, an account id, or a member's exact email address"`
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
			return nil, refusal(
				"state_needs_team",
				"filtering by state name needs the team parameter as well",
			)
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
		accountID, err := t.resolveAssignee(ctx, workspaceID, input.Assignee)
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
		cycleID, err := parseCycle(input.Cycle)
		if err != nil {
			return nil, err
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
	Assignee    string   `json:"assignee,omitempty" jsonschema:"me for the person this connection acts for, an account id, or a member's exact email address"`
	Project     string   `json:"project,omitempty" jsonschema:"a project slug or id to raise the issue in"`
	Cycle       string   `json:"cycle,omitempty" jsonschema:"a cycle id from norn_list_cycles to raise the issue in"`
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
		accountID, err := t.resolveAssignee(ctx, workspace.ID, input.Assignee)
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

	if input.Cycle != "" {
		cycleID, err := parseCycle(input.Cycle)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		create.CycleID = cycleID
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
	Workspace               string   `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue                   string   `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	ExpectedVersion         int      `json:"expected_version,omitempty" jsonschema:"the version returned by norn_get_issue; read automatically when omitted"`
	Title                   *string  `json:"title,omitempty" jsonschema:"a new title"`
	Description             *string  `json:"description,omitempty" jsonschema:"a new description, markdown"`
	Priority                *string  `json:"priority,omitempty" jsonschema:"urgent, high, medium, low, or none"`
	Assignee                *string  `json:"assignee,omitempty" jsonschema:"me for the person this connection acts for, an account id, or a member's exact email address"`
	Estimate                *int     `json:"estimate,omitempty" jsonschema:"a new effort estimate in points"`
	DueOn                   *string  `json:"due_on,omitempty" jsonschema:"a new due date as YYYY-MM-DD"`
	State                   *string  `json:"state,omitempty" jsonschema:"a workflow state name or id"`
	AcknowledgeOpenChildren bool     `json:"acknowledge_open_children,omitempty" jsonschema:"close the issue even though it still has open sub-issues"`
	Project                 *string  `json:"project,omitempty" jsonschema:"a project slug or id; to take the issue out of its project, name project in clear instead"`
	Cycle                   *string  `json:"cycle,omitempty" jsonschema:"a cycle id from norn_list_cycles; to take the issue out of its cycle, name cycle in clear instead"`
	Team                    *string  `json:"team,omitempty" jsonschema:"move the issue to this team key or id; the issue keeps its reference"`
	DropStrandedLabels      *bool    `json:"drop_stranded_labels,omitempty" jsonschema:"allow a move that loses labels the destination team cannot hold"`
	Labels                  []string `json:"labels,omitempty" jsonschema:"replace every label on the issue with these names or ids; cannot be combined with add_labels or remove_labels"`
	AddLabels               []string `json:"add_labels,omitempty" jsonschema:"label names or ids to put on the issue, keeping the labels it already has"`
	RemoveLabels            []string `json:"remove_labels,omitempty" jsonschema:"label names or ids to take off the issue, keeping the rest"`
	Clear                   []string `json:"clear,omitempty" jsonschema:"fields to clear: assignee, estimate, dueOn, cycle, or project"`
}

func (t *toolset) updateIssue(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input updateIssueInput,
) (*mcp.CallToolResult, issueOutput, error) {
	if err := updateConflicts(input); err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	current := issue

	version := input.ExpectedVersion
	if version == 0 {
		version = issue.Version
	}

	if input.Team != nil {
		team, err := t.resolveTeam(ctx, workspace.ID, *input.Team)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		move := service.MoveIssueInput{
			ExpectedVersion: version,
			TeamID:          team.ID,
		}

		if input.DropStrandedLabels != nil {
			move.AcknowledgeLabelLoss = *input.DropStrandedLabels
		}

		moved, err := t.issues.MoveToTeam(ctx, workspace.ID, issue.ID, move)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		current = moved
		version = moved.Version
	}

	if changesBeyondTeam(input) {
		update, err := t.updateFor(ctx, workspace.ID, current, input)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		update.ExpectedVersion = version

		updated, err := t.issues.Update(ctx, workspace.ID, issue.ID, update)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		current = updated
		version = updated.Version
	}

	if changesLabels(input) {
		labelIDs, err := t.nextLabels(ctx, workspace.ID, current, input)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		labelled, err := t.issues.SetLabels(ctx, workspace.ID, issue.ID, service.SetIssueLabelsInput{
			ExpectedVersion: version,
			LabelIDs:        labelIDs,
		})
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		current = labelled
	}

	return nil, issueOutput{Issue: issueDTOFrom(current)}, nil
}

func (t *toolset) updateFor(
	ctx context.Context,
	workspaceID uuid.UUID,
	issue entity.Issue,
	input updateIssueInput,
) (service.UpdateIssueInput, error) {
	update := service.UpdateIssueInput{
		AcknowledgeOpenChildren: input.AcknowledgeOpenChildren,
		Title:                   input.Title,
		Description:             input.Description,
		DueOn:                   input.DueOn,
		Clear:                   input.Clear,
	}

	if input.Priority != nil {
		priority := entity.IssuePriority(*input.Priority)
		update.Priority = &priority
	}

	if input.Estimate != nil {
		update.Estimate = input.Estimate
	}

	if input.Assignee != nil {
		accountID, err := t.resolveAssignee(ctx, workspaceID, *input.Assignee)
		if err != nil {
			return service.UpdateIssueInput{}, err
		}

		update.AssigneeID = &accountID
	}

	if input.State != nil {
		state, err := t.resolveState(ctx, workspaceID, issue.TeamID, *input.State)
		if err != nil {
			return service.UpdateIssueInput{}, err
		}

		stateID := state.ID
		update.StateID = &stateID
	}

	if input.Project != nil {
		project, err := t.resolveProject(ctx, workspaceID, *input.Project)
		if err != nil {
			return service.UpdateIssueInput{}, err
		}

		projectID := project.Project.ID
		update.ProjectID = &projectID
	}

	if input.Cycle != nil {
		cycleID, err := parseCycle(*input.Cycle)
		if err != nil {
			return service.UpdateIssueInput{}, err
		}

		update.CycleID = &cycleID
	}

	return update, nil
}

func (t *toolset) nextLabels(
	ctx context.Context,
	workspaceID uuid.UUID,
	issue entity.Issue,
	input updateIssueInput,
) ([]uuid.UUID, error) {
	if input.Labels != nil {
		return t.resolveLabels(ctx, workspaceID, input.Labels)
	}

	labels, err := t.labels.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	added, err := matchLabels(labels, input.AddLabels)
	if err != nil {
		return nil, err
	}

	removed, err := matchLabels(labels, input.RemoveLabels)
	if err != nil {
		return nil, err
	}

	for _, labelID := range added {
		if slices.Contains(removed, labelID) {
			return nil, errLabelAddedAndRemoved
		}
	}

	next := make([]uuid.UUID, 0, len(issue.Labels)+len(added))

	for _, label := range issue.Labels {
		if !slices.Contains(removed, label.ID) && !slices.Contains(next, label.ID) {
			next = append(next, label.ID)
		}
	}

	for _, labelID := range added {
		if !slices.Contains(next, labelID) {
			next = append(next, labelID)
		}
	}

	return next, nil
}

var errLabelAddedAndRemoved = refusal(
	"label_added_and_removed",
	"the same label is named in both add_labels and remove_labels; name it in only one of them",
)

func updateConflicts(input updateIssueInput) error {
	if input.Labels != nil && (len(input.AddLabels) > 0 || len(input.RemoveLabels) > 0) {
		return refusal(
			"labels_conflict",
			"labels replaces every label on the issue and cannot be combined with add_labels or "+
				"remove_labels; send either labels, or add_labels and remove_labels",
		)
	}

	for _, added := range input.AddLabels {
		for _, removed := range input.RemoveLabels {
			if strings.EqualFold(strings.TrimSpace(added), strings.TrimSpace(removed)) {
				return errLabelAddedAndRemoved
			}
		}
	}

	if input.Project != nil && clears(input.Clear, "project") {
		return refusal(
			"project_set_and_cleared",
			"project is given and also named in clear; send project to change it, or clear it, not both",
		)
	}

	if input.Cycle != nil && clears(input.Clear, "cycle") {
		return refusal(
			"cycle_set_and_cleared",
			"cycle is given and also named in clear; send cycle to change it, or clear it, not both",
		)
	}

	return nil
}

func clears(fields []string, name string) bool {
	return slices.ContainsFunc(fields, func(field string) bool {
		return strings.EqualFold(strings.TrimSpace(field), name)
	})
}

func parseCycle(ref string) (uuid.UUID, error) {
	cycleID, err := uuid.Parse(strings.TrimSpace(ref))
	if err != nil {
		return uuid.Nil, refusal(
			"cycle_invalid",
			fmt.Sprintf("cycle must be a cycle id from norn_list_cycles, not %q", ref),
		)
	}

	return cycleID, nil
}

type changeIssueStateInput struct {
	Workspace               string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue                   string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	State                   string `json:"state" jsonschema:"the target workflow state name or id"`
	ExpectedVersion         int    `json:"expected_version,omitempty" jsonschema:"the version from norn_get_issue; read automatically when omitted"`
	AcknowledgeOpenChildren bool   `json:"acknowledge_open_children,omitempty" jsonschema:"close the issue even though it still has open sub-issues"`
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
		ExpectedVersion:         version,
		StateID:                 &stateID,
		AcknowledgeOpenChildren: input.AcknowledgeOpenChildren,
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
		input.Cycle != nil ||
		len(input.Clear) > 0
}

func changesLabels(input updateIssueInput) bool {
	return input.Labels != nil || len(input.AddLabels) > 0 || len(input.RemoveLabels) > 0
}

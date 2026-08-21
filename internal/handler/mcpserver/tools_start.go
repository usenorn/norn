package mcpserver

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const startReminder = "Work on the branch above. Any question below is one an agent asked and " +
	"nobody has answered; the default it recorded is what stands until somebody does."

type startIssueInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue     string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
}

type startIssueOutput struct {
	Issue     issueDTO      `json:"issue"`
	Branch    string        `json:"branch"`
	Questions []questionDTO `json:"questions"`
	Reminder  string        `json:"reminder"`
}

func (t *toolset) startIssue(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input startIssueInput,
) (*mcp.CallToolResult, startIssueOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, startIssueOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, startIssueOutput{}, toolFailure(ctx, err)
	}

	started := issue

	if issue.State.Category != entity.StateCategoryActive {
		active, err := t.firstActiveState(ctx, workspace.ID, issue.TeamID)
		if err != nil {
			return nil, startIssueOutput{}, toolFailure(ctx, err)
		}

		started, err = t.issues.Update(ctx, workspace.ID, issue.ID, service.UpdateIssueInput{
			ExpectedVersion: issue.Version,
			StateID:         &active.ID,
		})
		if err != nil {
			return nil, startIssueOutput{}, toolFailure(ctx, err)
		}
	}

	branch, err := t.sourceControl.BranchName(ctx, workspace.ID, issue.ID)
	if err != nil {
		return nil, startIssueOutput{}, toolFailure(ctx, err)
	}

	asked, err := t.questions.List(ctx, workspace.ID, issue.ID)
	if err != nil {
		return nil, startIssueOutput{}, toolFailure(ctx, err)
	}

	return nil, startIssueOutput{
		Issue:     issueDTOFrom(started),
		Branch:    branch,
		Questions: questionDTOs(entity.UnansweredQuestions(asked)),
		Reminder:  startReminder,
	}, nil
}

func (t *toolset) firstActiveState(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.WorkflowState, error) {
	states, err := t.workflowStates.List(ctx, workspaceID, teamID)
	if err != nil {
		return entity.WorkflowState{}, err
	}

	active := make([]entity.WorkflowState, 0, len(states))

	for _, state := range states {
		if state.Category == entity.StateCategoryActive {
			active = append(active, state)
		}
	}

	if len(active) == 0 {
		return entity.WorkflowState{}, entity.ErrWorkflowStateNotFound
	}

	slices.SortFunc(active, func(a, b entity.WorkflowState) int {
		return a.Position - b.Position
	})

	return active[0], nil
}

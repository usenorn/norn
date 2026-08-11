package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type issueBranchNameInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue     string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
}

type issueBranchNameOutput struct {
	Branch string `json:"branch"`
}

func (t *toolset) issueBranchName(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input issueBranchNameInput,
) (*mcp.CallToolResult, issueBranchNameOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, issueBranchNameOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, issueBranchNameOutput{}, toolFailure(ctx, err)
	}

	branch, err := t.sourceControl.BranchName(ctx, workspace.ID, issue.ID)
	if err != nil {
		return nil, issueBranchNameOutput{}, toolFailure(ctx, err)
	}

	return nil, issueBranchNameOutput{Branch: branch}, nil
}

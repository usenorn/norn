package mcpserver

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

var issueStatusNames = map[string]entity.IssueStatus{
	"active":           entity.IssueStatusActive,
	"archived":         entity.IssueStatusArchived,
	"deleted":          entity.IssueStatusPendingDeletion,
	"pending_deletion": entity.IssueStatusPendingDeletion,
}

type setIssueStatusInput struct {
	Workspace       string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue           string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	Status          string `json:"status" jsonschema:"active to restore the issue, archived, or deleted; a deleted issue can be restored for 30 days"`
	ExpectedVersion int    `json:"expected_version,omitempty" jsonschema:"the version from norn_get_issue; read automatically when omitted"`
}

func (t *toolset) setIssueStatus(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input setIssueStatusInput,
) (*mcp.CallToolResult, issueOutput, error) {
	status, known := issueStatusNames[strings.ToLower(strings.TrimSpace(input.Status))]
	if !known {
		return nil, issueOutput{}, toolFailure(ctx, refusal(
			"status_invalid",
			"status must be active to restore the issue, archived, or deleted",
		))
	}

	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	version := input.ExpectedVersion
	if version == 0 {
		version = issue.Version
	}

	changed, err := t.issues.SetStatus(ctx, workspace.ID, issue.ID, service.SetIssueStatusInput{
		ExpectedVersion: version,
		Status:          status,
	})
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	return nil, issueOutput{Issue: issueDTOFrom(changed)}, nil
}

type setIssueParentInput struct {
	Workspace       string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue           string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	Parent          string `json:"parent" jsonschema:"the reference or id of the issue to file it under; an empty string takes it out from under its parent"`
	ExpectedVersion int    `json:"expected_version,omitempty" jsonschema:"the version from norn_get_issue; read automatically when omitted"`
}

func (t *toolset) setIssueParent(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input setIssueParentInput,
) (*mcp.CallToolResult, issueOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	var parentID *uuid.UUID

	if strings.TrimSpace(input.Parent) != "" {
		parent, err := t.resolveIssue(ctx, workspace.ID, input.Parent)
		if err != nil {
			return nil, issueOutput{}, toolFailure(ctx, err)
		}

		parentID = &parent.ID
	}

	version := input.ExpectedVersion
	if version == 0 {
		version = issue.Version
	}

	filed, err := t.issues.SetParent(ctx, workspace.ID, issue.ID, service.SetIssueParentInput{
		ExpectedVersion: version,
		ParentID:        parentID,
	})
	if err != nil {
		return nil, issueOutput{}, toolFailure(ctx, err)
	}

	return nil, issueOutput{Issue: issueDTOFrom(filed)}, nil
}

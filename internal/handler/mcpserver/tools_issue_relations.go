package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type linkIssuesInput struct {
	Workspace      string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue          string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	Related        string `json:"related" jsonschema:"the reference or id of the other issue"`
	Kind           string `json:"kind" jsonschema:"how issue stands to related: blocks, blocked_by, duplicates, duplicated_by, or relates_to"`
	CloseDuplicate bool   `json:"close_duplicate,omitempty" jsonschema:"with duplicates or duplicated_by, also move the duplicate into its team's abandoned state"`
}

type unlinkIssuesInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue     string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	Related   string `json:"related" jsonschema:"the reference or id of the issue it is related to"`
}

type relationOutput struct {
	Relation relationDTO `json:"relation"`
}

func (t *toolset) linkIssues(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input linkIssuesInput,
) (*mcp.CallToolResult, relationOutput, error) {
	kind := entity.IssueRelationView(strings.ToLower(strings.TrimSpace(input.Kind)))
	if !kind.Valid() {
		return nil, relationOutput{}, toolFailure(ctx, refusal(
			"relation_kind_invalid",
			"kind must be blocks, blocked_by, duplicates, duplicated_by, or relates_to",
		))
	}

	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, relationOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, relationOutput{}, toolFailure(ctx, err)
	}

	related, err := t.resolveIssue(ctx, workspace.ID, input.Related)
	if err != nil {
		return nil, relationOutput{}, toolFailure(ctx, err)
	}

	relation, err := t.relations.Add(ctx, workspace.ID, issue.ID, service.AddIssueRelationInput{
		Kind:           kind,
		CounterpartID:  related.ID,
		CloseDuplicate: input.CloseDuplicate,
	})
	if err != nil {
		return nil, relationOutput{}, toolFailure(ctx, err)
	}

	return nil, relationOutput{Relation: relationDTOFrom(relation)}, nil
}

func (t *toolset) unlinkIssues(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input unlinkIssuesInput,
) (*mcp.CallToolResult, relationOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, relationOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, relationOutput{}, toolFailure(ctx, err)
	}

	related, err := t.resolveIssue(ctx, workspace.ID, input.Related)
	if err != nil {
		return nil, relationOutput{}, toolFailure(ctx, err)
	}

	groups, err := t.relations.List(ctx, workspace.ID, issue.ID)
	if err != nil {
		return nil, relationOutput{}, toolFailure(ctx, err)
	}

	for _, group := range groups {
		for _, relation := range group.Relations {
			if relation.Issue.ID != related.ID {
				continue
			}

			if err := t.relations.Remove(ctx, workspace.ID, issue.ID, relation.ID); err != nil {
				return nil, relationOutput{}, toolFailure(ctx, err)
			}

			return nil, relationOutput{Relation: relationDTOFrom(relation)}, nil
		}
	}

	return nil, relationOutput{}, toolFailure(ctx, entity.ErrIssueRelationNotFound)
}

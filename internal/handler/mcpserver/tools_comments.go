package mcpserver

import (
	"context"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/service"
)

type createCommentInput struct {
	Workspace       string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue           string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	Body            string `json:"body" jsonschema:"the comment body, markdown"`
	ParentCommentID string `json:"parent_comment_id,omitempty" jsonschema:"reply to this comment id"`
}

type createCommentOutput struct {
	Comment commentDTO `json:"comment"`
}

func (t *toolset) createComment(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input createCommentInput,
) (*mcp.CallToolResult, createCommentOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, createCommentOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, createCommentOutput{}, toolFailure(ctx, err)
	}

	post := service.PostCommentInput{Body: input.Body}

	if input.ParentCommentID != "" {
		parentID, err := uuid.Parse(input.ParentCommentID)
		if err != nil {
			return nil, createCommentOutput{}, toolFailure(ctx, err)
		}

		post.ParentCommentID = parentID
	}

	posted, err := t.issueComments.Post(ctx, workspace.ID, issue.ID, post)
	if err != nil {
		return nil, createCommentOutput{}, toolFailure(ctx, err)
	}

	return nil, createCommentOutput{Comment: commentDTOFrom(posted.Comment)}, nil
}

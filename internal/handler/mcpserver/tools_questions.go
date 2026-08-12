package mcpserver

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const askReminder = "You are not waiting for this. Carry on with the default you declared. If " +
	"nobody answers before the deadline the default is what stands, and finishing the issue " +
	"with the question still unanswered puts the close in front of a person to ratify."

type askInput struct {
	Workspace   string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue       string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	Question    string `json:"question" jsonschema:"what you need a person to decide, in one question"`
	Default     string `json:"default" jsonschema:"what you will do if nobody answers before the deadline"`
	WaitSeconds int    `json:"wait_seconds,omitempty" jsonschema:"how long to leave the question open before the default stands; defaults to a day"`
}

type askOutput struct {
	Question questionDTO `json:"question"`
	Reminder string      `json:"reminder"`
}

func (t *toolset) ask(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input askInput,
) (*mcp.CallToolResult, askOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, askOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, askOutput{}, toolFailure(ctx, err)
	}

	asked, err := t.questions.Ask(ctx, workspace.ID, issue.ID, service.AskQuestionInput{
		Question: input.Question,
		Default:  input.Default,
		Wait:     askWait(input.WaitSeconds),
	})
	if err != nil {
		return nil, askOutput{}, toolFailure(ctx, err)
	}

	return nil, askOutput{Question: questionDTOFrom(asked), Reminder: askReminder}, nil
}

func askWait(seconds int) time.Duration {
	if seconds <= 0 {
		return entity.QuestionWaitDefault
	}

	return time.Duration(seconds) * time.Second
}

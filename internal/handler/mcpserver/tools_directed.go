package mcpserver

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
)

type listDirectedInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Recipient string `json:"recipient" jsonschema:"the account id of the person you sent things to"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size, at most 100"`
}

type directedNoticeDTO struct {
	Issue     string `json:"issue,omitempty"`
	Title     string `json:"title,omitempty"`
	Reason    string `json:"reason"`
	SentAt    string `json:"sentAt"`
	Inbox     bool   `json:"inbox"`
	Email     bool   `json:"email"`
	Cleared   bool   `json:"cleared"`
	ClearedAt string `json:"clearedAt,omitempty"`
	Opened    bool   `json:"opened"`
	OpenedAt  string `json:"openedAt,omitempty"`
}

type listDirectedOutput struct {
	Notices []directedNoticeDTO `json:"notices"`
}

func (t *toolset) listDirected(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input listDirectedInput,
) (*mcp.CallToolResult, listDirectedOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, listDirectedOutput{}, toolFailure(ctx, err)
	}

	recipientID, err := uuid.Parse(input.Recipient)
	if err != nil {
		return nil, listDirectedOutput{}, toolFailure(ctx, err)
	}

	notices, err := t.notifications.Directed(ctx, workspace.ID, recipientID, uuid.Nil, input.Limit)
	if err != nil {
		return nil, listDirectedOutput{}, toolFailure(ctx, err)
	}

	out := listDirectedOutput{Notices: make([]directedNoticeDTO, 0, len(notices))}

	for _, notice := range notices {
		out.Notices = append(out.Notices, directedNotice(notice))
	}

	return nil, out, nil
}

func directedNotice(notice entity.DirectedNotice) directedNoticeDTO {
	dto := directedNoticeDTO{
		Issue:   notice.Reference,
		Title:   notice.Title,
		Reason:  string(notice.Reason),
		SentAt:  notice.SentAt.UTC().Format(time.RFC3339),
		Inbox:   notice.Channels.Inbox,
		Email:   notice.Channels.Email,
		Cleared: notice.Cleared(),
		Opened:  notice.Opened(),
	}

	if dto.Cleared {
		dto.ClearedAt = notice.ClearedAt.UTC().Format(time.RFC3339)
	}

	if dto.Opened {
		dto.OpenedAt = notice.OpenedAt.UTC().Format(time.RFC3339)
	}

	return dto
}

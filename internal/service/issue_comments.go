package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_comments.go -destination=issuecomment/mock_issue_comments.go -package=issuecomment -mock_names=IssueComments=MockIssueComments

type ListCommentsInput struct {
	Limit  int
	Cursor string
}

type CommentThread struct {
	Comments   []entity.IssueComment
	NextCursor string
}

type CommentMentionInput struct {
	Kind      entity.MentionKind
	AccountID uuid.UUID
	TeamID    uuid.UUID
}

type PostCommentInput struct {
	ParentCommentID uuid.UUID
	Body            string
	Mentions        []CommentMentionInput
	AttachmentIDs   []uuid.UUID
}

type CommentPosted struct {
	Comment     entity.IssueComment
	Unreachable []entity.CommentMention
}

type EditCommentInput struct {
	Body string
}

type IssueComments interface {
	List(ctx context.Context, workspaceID, issueID uuid.UUID, input ListCommentsInput) (CommentThread, error)
	Post(ctx context.Context, workspaceID, issueID uuid.UUID, input PostCommentInput) (CommentPosted, error)
	Edit(ctx context.Context, workspaceID, issueID, commentID uuid.UUID, input EditCommentInput) (entity.IssueComment, error)
	Remove(ctx context.Context, workspaceID, issueID, commentID uuid.UUID) error
	React(ctx context.Context, workspaceID, issueID, commentID uuid.UUID, reaction entity.CommentReaction) (entity.IssueComment, error)
	Unreact(ctx context.Context, workspaceID, issueID, commentID uuid.UUID, reaction entity.CommentReaction) (entity.IssueComment, error)
}

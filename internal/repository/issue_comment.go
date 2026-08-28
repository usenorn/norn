package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_comment.go -destination=issuecomment/mock_issue_comment.go -package=issuecomment -mock_names=IssueComment=MockIssueComment

type CommentAudience struct {
	AccountID uuid.UUID
	Name      string
	Visible   bool
}

type IssueComment interface {
	Create(ctx context.Context, comment entity.IssueComment) (entity.IssueComment, error)
	GetByID(ctx context.Context, workspaceID, commentID uuid.UUID) (entity.IssueComment, error)
	LockByID(ctx context.Context, workspaceID, commentID uuid.UUID) (entity.IssueComment, error)
	ListThread(
		ctx context.Context,
		issueID, readerID uuid.UUID,
		page entity.CommentPage,
	) ([]entity.IssueComment, error)
	CursorBefore(ctx context.Context, issueID, commentID uuid.UUID) (*entity.CommentCursor, error)
	Edit(ctx context.Context, commentID uuid.UUID, body string, at time.Time) error
	Tombstone(ctx context.Context, commentID uuid.UUID, at time.Time) error
	PurgeImported(ctx context.Context, workspaceID uuid.UUID, ids []uuid.UUID) error
	RecordMentions(ctx context.Context, commentID uuid.UUID, mentions []entity.CommentMention) error
	Mentioned(ctx context.Context, commentID uuid.UUID) ([]entity.CommentMention, error)
	Audience(ctx context.Context, workspaceID, teamID uuid.UUID, accountIDs []uuid.UUID) ([]CommentAudience, error)
	React(ctx context.Context, commentID, accountID uuid.UUID, reaction entity.CommentReaction) error
	Unreact(ctx context.Context, commentID, accountID uuid.UUID, reaction entity.CommentReaction) error
}

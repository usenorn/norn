package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_review.go -destination=executionreview/mock_execution_review.go -package=executionreview -mock_names=ExecutionReview=MockExecutionReview

type ReviewCommentEdit struct {
	ExecutionID string
	CommentID   uuid.UUID
	Body        string
	At          time.Time
}

type ReviewResolution struct {
	ExecutionID string
	CommentID   uuid.UUID
	AccountID   uuid.UUID
	Resolved    bool
	At          time.Time
}

type ExecutionReview interface {
	AddComment(
		ctx context.Context, comment entity.ExecutionReviewComment,
	) (entity.ExecutionReviewComment, error)
	GetComment(
		ctx context.Context, executionID string, commentID uuid.UUID,
	) (entity.ExecutionReviewComment, error)
	ListComments(ctx context.Context, executionID string) ([]entity.ExecutionReviewComment, error)
	CountComments(ctx context.Context, executionID string) (int, error)
	EditComment(ctx context.Context, edit ReviewCommentEdit) (entity.ExecutionReviewComment, error)
	DeleteComment(ctx context.Context, executionID string, commentID uuid.UUID) error
	ResolveComment(
		ctx context.Context, resolution ReviewResolution,
	) (entity.ExecutionReviewComment, error)
	CreateReview(ctx context.Context, review entity.ExecutionReview) (entity.ExecutionReview, error)
	AttachPending(ctx context.Context, executionID string, authorID, reviewID uuid.UUID) error
	ListReviews(ctx context.Context, executionID string) ([]entity.ExecutionReview, error)
}

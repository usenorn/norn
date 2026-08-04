package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=attachment.go -destination=attachment/mock_attachment.go -package=attachment -mock_names=Attachment=MockAttachment

type Attachment interface {
	Create(ctx context.Context, attachment entity.Attachment) (entity.Attachment, error)
	GetByID(ctx context.Context, workspaceID, attachmentID uuid.UUID) (entity.Attachment, error)
	LockByID(ctx context.Context, workspaceID, attachmentID uuid.UUID) (entity.Attachment, error)
	ListByIssue(ctx context.Context, issueID uuid.UUID) ([]entity.Attachment, error)
	Settle(ctx context.Context, attachmentID uuid.UUID, sizeBytes int64, contentType string, at time.Time) error
	Discard(ctx context.Context, attachmentID uuid.UUID, at time.Time) error
	ClaimForComment(ctx context.Context, workspaceID, issueID, commentID uuid.UUID, attachmentIDs []uuid.UUID) error
	MarkOrphans(ctx context.Context, at time.Time) error
	ListReclaimable(ctx context.Context, at time.Time, batch int) ([]entity.Attachment, error)
	Reclaim(ctx context.Context, attachmentID uuid.UUID) error
	Admit(ctx context.Context, workspaceID uuid.UUID, sizeBytes, maxBytes int64) (int64, error)
	Release(ctx context.Context, workspaceID uuid.UUID, sizeBytes int64) error
	Ledger(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceStorage, error)
}

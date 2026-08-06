package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=attachments.go -destination=attachment/mock_attachments.go -package=attachment -mock_names=Attachments=MockAttachments

type ReserveAttachmentInput struct {
	FileName    string
	ContentType string
	SizeBytes   int64
}

type AdoptAttachmentInput struct {
	ObjectKey   string
	FileName    string
	ContentType string
	SizeBytes   int64
	CommentID   uuid.UUID
	Origin      *entity.ImportOrigin
}

type AttachmentReservation struct {
	Attachment entity.Attachment
	Transfer   entity.BlobTicket
}

type Attachments interface {
	List(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.Attachment, error)
	Reserve(ctx context.Context, workspaceID, issueID uuid.UUID, input ReserveAttachmentInput) (AttachmentReservation, error)
	Adopt(ctx context.Context, workspaceID, issueID uuid.UUID, input AdoptAttachmentInput) (entity.Attachment, error)
	Finalize(ctx context.Context, workspaceID, issueID, attachmentID uuid.UUID) (entity.Attachment, error)
	Remove(ctx context.Context, workspaceID, issueID, attachmentID uuid.UUID) error
	Content(ctx context.Context, workspaceID, attachmentID uuid.UUID) (string, error)
	Ledger(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceStorage, error)
	Reclaim(ctx context.Context) error
}

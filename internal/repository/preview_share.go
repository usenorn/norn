package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=preview_share.go -destination=previewshare/mock_preview_share.go -package=previewshare -mock_names=PreviewShare=MockPreviewShare

type PreviewShare interface {
	Create(ctx context.Context, link entity.PreviewShareLink) (entity.PreviewShareLink, error)
	ByToken(ctx context.Context, tokenHash []byte) (entity.PreviewShareLink, error)
	ByPreview(ctx context.Context, previewID uuid.UUID) ([]entity.PreviewShareLink, error)
	ByExecution(ctx context.Context, executionID string) ([]entity.PreviewShareLink, error)
	Revoke(
		ctx context.Context,
		previewID, linkID uuid.UUID,
		revokedAt time.Time,
	) (entity.PreviewShareLink, error)
	Used(ctx context.Context, linkID uuid.UUID, usedAt time.Time) error
}

package repository

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=preview.go -destination=preview/mock_preview.go -package=preview -mock_names=Preview=MockPreview

type Preview interface {
	Save(ctx context.Context, preview entity.PreviewSession) (entity.PreviewSession, error)
	ByHost(ctx context.Context, host string) (entity.PreviewSession, error)
	ByName(ctx context.Context, executionID, name string) (entity.PreviewSession, error)
	ByExecution(ctx context.Context, executionID string) ([]entity.PreviewSession, error)
	Count(ctx context.Context, executionID string) (int, error)
	CloseByExecution(ctx context.Context, executionID string, closedAt time.Time) error
}

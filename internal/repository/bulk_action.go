package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=bulk_action.go -destination=bulkaction/mock_bulk_action.go -package=bulkaction -mock_names=BulkAction=MockBulkAction

type BulkAction interface {
	Create(ctx context.Context, action entity.BulkAction) (entity.BulkAction, error)
	GetByID(ctx context.Context, workspaceID, actionID uuid.UUID) (entity.BulkAction, error)
	Claim(ctx context.Context, actionID uuid.UUID, at time.Time) error
	Advance(ctx context.Context, actionID uuid.UUID, by int) error
	Settle(ctx context.Context, actionID uuid.UUID, status entity.BulkActionStatus, at time.Time) error
	RecordOutcomes(ctx context.Context, actionID uuid.UUID, outcomes []entity.BulkActionOutcome) error
	ListOutcomes(ctx context.Context, actionID uuid.UUID, scope entity.TeamScope) ([]entity.BulkActionOutcome, error)
}

package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=bulk_operations.go -destination=bulkoperation/mock_bulk_operations.go -package=bulkoperation -mock_names=BulkOperations=MockBulkOperations

type BulkOperations interface {
	Apply(ctx context.Context, workspaceID uuid.UUID, input ApplyBulkInput) (BulkResult, error)
	Run(ctx context.Context, payload entity.BulkApplyPayload) error
	Get(ctx context.Context, workspaceID, actionID uuid.UUID) (entity.BulkAction, []entity.BulkActionOutcome, error)
}

type ApplyBulkInput struct {
	Change entity.BulkChange
	Set    entity.BulkSet
}

type BulkResult struct {
	Action   *entity.BulkAction
	Outcomes []entity.BulkActionOutcome
}

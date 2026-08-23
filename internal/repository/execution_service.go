package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_service.go -destination=executionservice/mock_execution_service.go -package=executionservice -mock_names=ExecutionService=MockExecutionService

type ExecutionService interface {
	Save(ctx context.Context, service entity.ExecutionService) (entity.ExecutionService, error)
	ByExecution(ctx context.Context, executionID string) ([]entity.ExecutionService, error)
	Count(ctx context.Context, executionID string) (int, error)
}

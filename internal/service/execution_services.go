package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_services.go -destination=executionservice/mock_execution_services.go -package=executionservice -mock_names=ExecutionServices=MockExecutionServices

type ExecutionServices interface {
	Reported(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error

	ForExecution(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
	) ([]entity.ExecutionService, error)
}

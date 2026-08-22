package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_policy.go -destination=executionpolicy/mock_execution_policy.go -package=executionpolicy -mock_names=ExecutionPolicy=MockExecutionPolicy

type ExecutionPolicy interface {
	Policy(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceExecutionPolicy, error)
	Upsert(
		ctx context.Context,
		policy entity.WorkspaceExecutionPolicy,
	) (entity.WorkspaceExecutionPolicy, error)
}

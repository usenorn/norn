package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_snapshot.go -destination=executionsnapshot/mock_execution_snapshot.go -package=executionsnapshot -mock_names=ExecutionSnapshot=MockExecutionSnapshot

type ExecutionSnapshot interface {
	Record(ctx context.Context, snapshot entity.ExecutionSnapshot) (entity.ExecutionSnapshot, error)
	Latest(ctx context.Context, executionID string) (entity.ExecutionSnapshot, error)
	ByRevision(ctx context.Context, executionID string, revision int) (entity.ExecutionSnapshot, error)
	Revisions(ctx context.Context, executionID string) ([]entity.ExecutionRevision, error)
}

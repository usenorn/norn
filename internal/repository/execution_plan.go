package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution_plan.go -destination=executionplan/mock_execution_plan.go -package=executionplan -mock_names=ExecutionPlan=MockExecutionPlan

type PlanDecision struct {
	ExecutionID string
	Revision    int
	AccountID   uuid.UUID
	Feedback    string
	At          time.Time
}

type ExecutionPlan interface {
	Propose(ctx context.Context, plan entity.ExecutionPlan) (entity.ExecutionPlan, error)
	ListByExecution(ctx context.Context, executionID string) ([]entity.ExecutionPlan, error)
	Approve(ctx context.Context, decision PlanDecision) (entity.ExecutionPlan, error)
	RequestRevision(ctx context.Context, decision PlanDecision) (entity.ExecutionPlan, error)
}

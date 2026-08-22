package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=execution.go -destination=execution/mock_execution.go -package=execution -mock_names=Execution=MockExecution

type NewExecution struct {
	ID           string
	WorkspaceID  uuid.UUID
	IssueID      uuid.UUID
	DelegationID uuid.UUID
	AgentID      uuid.UUID
	RunnerID     uuid.UUID
	CodebaseID   uuid.UUID
	Attempt      int
	Params       entity.ExecutionParams
	QueuedAt     time.Time
}

type ExecutionMove struct {
	From           entity.ExecutionState
	To             entity.ExecutionState
	Reason         string
	RunnerID       uuid.UUID
	LeaseExpiresAt *time.Time
	At             time.Time
}

type Execution interface {
	Create(ctx context.Context, execution NewExecution) (entity.Execution, error)
	GetByID(ctx context.Context, executionID string) (entity.Execution, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.Execution, error)
	ListLiveByRunner(ctx context.Context, runnerID uuid.UUID) ([]entity.Execution, error)
	NextAttempt(ctx context.Context, issueID uuid.UUID) (int, error)
	Move(ctx context.Context, executionID string, move ExecutionMove) (entity.Execution, error)
	RenewLeases(ctx context.Context, runnerID uuid.UUID, expiresAt time.Time) error
	ExpiredLeases(ctx context.Context, now time.Time, limit int) ([]entity.Execution, error)
	AppendEvent(ctx context.Context, event entity.ExecutionEvent) (entity.ExecutionEvent, error)
	ListEvents(
		ctx context.Context, executionID string, page entity.ExecutionTimelinePage,
	) ([]entity.ExecutionEvent, error)
}

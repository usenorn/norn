package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=executions.go -destination=execution/mock_executions.go -package=execution -mock_names=Executions=MockExecutions

type ExecutionDetail struct {
	Execution entity.Execution
	Timeline  []entity.ExecutionEvent
}

type Executions interface {
	OnDelegated(ctx context.Context, issue entity.Issue, delegation entity.IssueDelegation) error

	Get(ctx context.Context, workspaceID uuid.UUID, executionID string) (ExecutionDetail, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.Execution, error)
	Timeline(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
		page entity.ExecutionTimelinePage,
	) ([]entity.ExecutionEvent, error)

	Cancel(ctx context.Context, workspaceID uuid.UUID, executionID, reason string) (entity.Execution, error)
	Restart(ctx context.Context, workspaceID uuid.UUID, executionID string) (entity.Execution, error)
	Resume(ctx context.Context, workspaceID uuid.UUID, executionID, feedback string) (entity.Execution, error)
	Approve(ctx context.Context, workspaceID uuid.UUID, executionID string) (entity.Execution, error)

	Accepted(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Declined(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Reported(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Observed(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Renew(ctx context.Context, runner entity.Runner) error
	Leased(ctx context.Context, runnerID uuid.UUID) ([]string, error)
	SweepLeases(ctx context.Context) error
}

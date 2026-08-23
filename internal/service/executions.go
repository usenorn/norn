package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=executions.go -destination=execution/mock_executions.go -package=execution -mock_names=Executions=MockExecutions

type ExecutionDetail struct {
	Execution entity.Execution
	Timeline  []entity.ExecutionEvent
	ChangeSet entity.ExecutionChangeSet
	Previews  []entity.PreviewSession
}

type RunnerReadiness struct {
	Runner       entity.Runner
	Connected    bool
	Reaches      bool
	Capacity     int
	Used         int
	Free         int
	DiskPressure bool
}

type ExecutionPlacement struct {
	Runners  []RunnerReadiness
	RunnerID uuid.UUID
	Waiting  entity.ExecutionQueuedReason
	Sharing  []entity.Execution
}

type Executions interface {
	OnDelegated(ctx context.Context, issue entity.Issue, delegation entity.IssueDelegation) error
	Placement(
		ctx context.Context, issue entity.Issue, agentID uuid.UUID,
	) (ExecutionPlacement, error)

	Get(ctx context.Context, workspaceID uuid.UUID, executionID string) (ExecutionDetail, error)
	Visible(ctx context.Context, workspaceID uuid.UUID, executionID string) (entity.Execution, error)
	Manageable(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
	) (entity.Execution, error)
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
	Retain(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
		longer time.Duration,
	) (entity.Execution, error)

	Questioned(ctx context.Context, question entity.IssueQuestion) error
	Answered(ctx context.Context, question entity.IssueQuestion) error
	Unanswerable(ctx context.Context, question entity.IssueQuestion, reason string) error

	Ready(ctx context.Context, runner entity.Runner) error
	Accepted(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Declined(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Reported(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Observed(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Held(ctx context.Context, runner entity.Runner, executionID string) (entity.Execution, error)
	Renew(ctx context.Context, runner entity.Runner) error
	Leased(ctx context.Context, runnerID uuid.UUID) ([]string, error)
	SweepLeases(ctx context.Context) error
}

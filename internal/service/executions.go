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
	Services  []entity.ExecutionService
	Machine   *RunnerState
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

type ExecutionReviewState struct {
	Heads    entity.ReviewHeads
	Comments []entity.ExecutionReviewComment
	Reviews  []entity.ExecutionReview
}

type ReviewCommentDraft struct {
	ParentID uuid.UUID
	Anchor   entity.ReviewAnchor
	Body     string
	Publish  bool
}

type ReviewSubmission struct {
	Verdict entity.ExecutionReviewVerdict
	Summary string
	Heads   entity.ReviewHeads
}

type DecisionRight struct {
	CanDecide bool
	MakerName string
}

type ReviewQuestion struct {
	Question entity.IssueQuestion
	Right    DecisionRight
}

type ReviewRun struct {
	Listing entity.ExecutionListing
	Right   DecisionRight
}

type ReviewQueue struct {
	Questions []ReviewQuestion
	Plans     []ReviewRun
	Changes   []ReviewRun
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
	List(
		ctx context.Context, workspaceID uuid.UUID, page entity.ExecutionPage,
	) ([]entity.ExecutionListing, error)
	Timeline(
		ctx context.Context,
		workspaceID uuid.UUID,
		executionID string,
		page entity.ExecutionTimelinePage,
	) ([]entity.ExecutionEvent, error)
	Queue(ctx context.Context, workspaceID uuid.UUID) (ReviewQueue, error)
	DecisionRight(ctx context.Context, workspaceID, issueID uuid.UUID) (DecisionRight, error)

	Cancel(ctx context.Context, workspaceID uuid.UUID, executionID, reason string) (entity.Execution, error)
	Restart(ctx context.Context, workspaceID uuid.UUID, executionID string) (entity.Execution, error)

	Plans(ctx context.Context, workspaceID uuid.UUID, executionID string) ([]entity.ExecutionPlan, error)
	ApprovePlan(
		ctx context.Context, workspaceID uuid.UUID, executionID string, revision int,
	) (entity.Execution, error)
	RevisePlan(
		ctx context.Context, workspaceID uuid.UUID, executionID string, revision int, feedback string,
	) (entity.Execution, error)

	Review(ctx context.Context, workspaceID uuid.UUID, executionID string) (ExecutionReviewState, error)
	CommentOnReview(
		ctx context.Context, workspaceID uuid.UUID, executionID string, draft ReviewCommentDraft,
	) (entity.ExecutionReviewComment, error)
	EditReviewComment(
		ctx context.Context, workspaceID uuid.UUID, executionID string, commentID uuid.UUID, body string,
	) (entity.ExecutionReviewComment, error)
	DeleteReviewComment(
		ctx context.Context, workspaceID uuid.UUID, executionID string, commentID uuid.UUID,
	) error
	ResolveReviewComment(
		ctx context.Context, workspaceID uuid.UUID, executionID string, commentID uuid.UUID, resolved bool,
	) (entity.ExecutionReviewComment, error)
	SubmitReview(
		ctx context.Context, workspaceID uuid.UUID, executionID string, submission ReviewSubmission,
	) (entity.ExecutionReview, error)

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
	PlanProposed(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Observed(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Kept(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Held(ctx context.Context, runner entity.Runner, executionID string) (entity.Execution, error)
	Renew(ctx context.Context, runner entity.Runner) error
	Leased(ctx context.Context, runnerID uuid.UUID) ([]string, error)
	SweepLeases(ctx context.Context) error
}

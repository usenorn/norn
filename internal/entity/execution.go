package entity

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	ExecutionIDPrefix = "exec-"

	ExecutionReasonMaxLen   = 2000
	ExecutionFeedbackMaxLen = 4000
	ExecutionToolMaxLen     = 64
	ExecutionModelMaxLen    = 128

	ExecutionTimelinePageDefaultSize = 100
	ExecutionTimelinePageMaxSize     = 500
	ExecutionTimelinePreview         = 50

	ExecutionListDefaultSize = 50
	ExecutionListMaxSize     = 200
)

var (
	ErrExecutionNotFound        = errors.New("execution not found")
	ErrExecutionTransition      = errors.New("execution cannot move between those states")
	ErrExecutionFinished        = errors.New("this execution has already finished")
	ErrExecutionUnfinished      = errors.New("this execution has not finished yet")
	ErrExecutionStateNotRunners = errors.New("a runner may not put an execution into that state")
	ErrExecutionNotReviewable   = errors.New("this execution is not waiting to be reviewed")
	ErrExecutionSelfApproval    = errors.New("an agent may not approve its own work")
	ErrExecutionNoRunner        = errors.New("this agent has no runner to hand the work to")
	ErrExecutionAlreadyLive     = errors.New("this delegation already has a run in flight")
	ErrExecutionNotDelegated    = errors.New("this issue is not delegated to an agent")
	ErrExecutionEventRecorded   = errors.New("this timeline entry has already been recorded")
	ErrExecutionLeaseLapsed     = errors.New("the runner stopped reporting and its lease lapsed")
)

type ExecutionState = channelv1.State

const (
	ExecutionQueued          = channelv1.StateQueued
	ExecutionLeased          = channelv1.StateLeased
	ExecutionPreparing       = channelv1.StatePreparing
	ExecutionRunning         = channelv1.StateRunning
	ExecutionWaitingForInput = channelv1.StateWaitingForInput
	ExecutionQueuedForResume = channelv1.StateQueuedForResume
	ExecutionFinalizing      = channelv1.StateFinalizing
	ExecutionAwaitingReview  = channelv1.StateAwaitingReview
	ExecutionApproved        = channelv1.StateApproved
	ExecutionCompleted       = channelv1.StateCompleted
	ExecutionFailed          = channelv1.StateFailed
	ExecutionCancelled       = channelv1.StateCancelled
	ExecutionInterrupted     = channelv1.StateInterrupted
)

func ExecutionStates() []ExecutionState {
	return channelv1.States()
}

func TerminalExecutionStates() []ExecutionState {
	return channelv1.TerminalStates()
}

func RunnerDrivenExecutionStates() []ExecutionState {
	return channelv1.RunnerDrivenStates()
}

func MovesTheIssue(state ExecutionState) bool {
	return state == ExecutionRunning || state == ExecutionAwaitingReview ||
		state == ExecutionCompleted
}

func IssueStateFor(state ExecutionState, states []WorkflowState) (WorkflowState, bool) {
	switch state {
	case ExecutionRunning:
		return FirstActiveState(states)
	case ExecutionAwaitingReview:
		return LastActiveState(states)
	case ExecutionCompleted:
		return CompletionState(states)
	default:
		return WorkflowState{}, false
	}
}

type ExecutionQueuedReason string

const (
	QueuedNoRunner       ExecutionQueuedReason = "no_runner"
	QueuedRunnersOffline ExecutionQueuedReason = "runners_offline"
	QueuedRunnersPaused  ExecutionQueuedReason = "runners_paused"
	QueuedRunnersBusy    ExecutionQueuedReason = "runners_busy"
)

func ExecutionQueuedReasons() []ExecutionQueuedReason {
	return []ExecutionQueuedReason{
		QueuedNoRunner, QueuedRunnersOffline, QueuedRunnersPaused, QueuedRunnersBusy,
	}
}

func (r ExecutionQueuedReason) Valid() bool {
	return slices.Contains(ExecutionQueuedReasons(), r)
}

type ExecutionParams struct {
	Tool         string
	Model        string
	Runtime      CodebaseRuntime
	BaseRef      BaseRefPolicy
	IncludeDirty bool
	Profile      PermissionProfile
	Brief        string
}

type Execution struct {
	ID             string
	WorkspaceID    uuid.UUID
	IssueID        uuid.UUID
	IssueReference string
	IssueTitle     string
	TeamID         uuid.UUID
	DelegationID   uuid.UUID
	AgentID        uuid.UUID
	AgentName      string
	RunnerID       uuid.UUID
	RunnerName     string
	CodebaseID     uuid.UUID
	CodebaseName   string
	Attempt        int
	State          ExecutionState
	Reason         string
	QueuedReason   ExecutionQueuedReason
	Params         ExecutionParams
	LeaseExpiresAt *time.Time
	KeepUntil      *time.Time
	QueuedAt       time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	UpdatedAt      time.Time
}

type ExecutionPage struct {
	States []ExecutionState
	Limit  int
}

func (p ExecutionPage) Size() int {
	if p.Limit <= 0 {
		return ExecutionListDefaultSize
	}

	return min(p.Limit, ExecutionListMaxSize)
}

func NewExecutionID(minted string) string {
	return ExecutionIDPrefix + minted
}

func (e Execution) Reference() string {
	if e.Attempt <= 1 {
		return e.IssueReference
	}

	return e.IssueReference + "-r" + strconv.Itoa(e.Attempt)
}

func (e Execution) Finished() bool {
	return e.State.Terminal()
}

func (e Execution) Restartable() bool {
	return e.State.Terminal() && e.State != ExecutionCompleted
}

func (e Execution) Stale(now time.Time) bool {
	return e.LeaseExpiresAt != nil && e.LeaseExpiresAt.Before(now)
}

func (e Execution) Underway(now time.Time) bool {
	return !e.Finished() && !e.Stale(now)
}

type ExecutionActor struct {
	Kind      ActorKind
	AccountID uuid.UUID
	AgentID   uuid.UUID
	RunnerID  uuid.UUID
}

func SystemExecutionActor() ExecutionActor {
	return ExecutionActor{Kind: ActorKindSystem}
}

func ExecutionActorOf(actor Actor) ExecutionActor {
	acting := ExecutionActor{Kind: actor.Kind, AccountID: actor.AccountID}

	if acting.Kind == "" {
		acting.Kind = ActorKindSystem
	}

	if actor.AgentID != nil {
		acting.AgentID = *actor.AgentID
	}

	if actor.RunnerID != nil {
		acting.RunnerID = *actor.RunnerID
	}

	return acting
}

type ExecutionEventKind = channelv1.EventKind

const (
	ExecutionEventTransition = channelv1.EventTransition
	ExecutionEventPhase      = channelv1.EventPhase
	ExecutionEventCommand    = channelv1.EventCommand
	ExecutionEventTool       = channelv1.EventTool
	ExecutionEventService    = channelv1.EventService
	ExecutionEventPreview    = channelv1.EventPreview
	ExecutionEventQuestion   = channelv1.EventQuestion
	ExecutionEventNote       = channelv1.EventNote
)

func ExecutionEventKinds() []ExecutionEventKind {
	return channelv1.EventKinds()
}

type ExecutionEvent struct {
	ID          uuid.UUID
	ExecutionID string
	Sequence    int64
	Kind        ExecutionEventKind
	FromState   ExecutionState
	ToState     ExecutionState
	Actor       ExecutionActor
	Reason      string
	Detail      []byte
	SourceID    string
	OccurredAt  time.Time
	RecordedAt  time.Time
}

type ExecutionTimelinePage struct {
	Limit int
	After int64
}

func (p ExecutionTimelinePage) Normalized() ExecutionTimelinePage {
	if p.Limit <= 0 {
		p.Limit = ExecutionTimelinePageDefaultSize
	}

	if p.Limit > ExecutionTimelinePageMaxSize {
		p.Limit = ExecutionTimelinePageMaxSize
	}

	if p.After < 0 {
		p.After = 0
	}

	return p
}

func ValidateExecutionReason(field, reason string) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(reason)) > ExecutionReasonMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateExecutionFeedback(field, feedback string) FieldError {
	length := utf8.RuneCountInString(strings.TrimSpace(feedback))

	switch {
	case length == 0:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > ExecutionFeedbackMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

package entity

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
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
	ErrExecutionNotDelegated    = errors.New("this issue is not delegated to an agent")
	ErrExecutionEventRecorded   = errors.New("this timeline entry has already been recorded")
	ErrExecutionLeaseLapsed     = errors.New("the runner stopped reporting and its lease lapsed")
)

type ExecutionState string

const (
	ExecutionQueued          ExecutionState = "queued"
	ExecutionLeased          ExecutionState = "leased"
	ExecutionPreparing       ExecutionState = "preparing"
	ExecutionRunning         ExecutionState = "running"
	ExecutionWaitingForInput ExecutionState = "waiting_for_input"
	ExecutionQueuedForResume ExecutionState = "queued_for_resume"
	ExecutionFinalizing      ExecutionState = "finalizing"
	ExecutionAwaitingReview  ExecutionState = "awaiting_review"
	ExecutionApproved        ExecutionState = "approved"
	ExecutionCompleted       ExecutionState = "completed"
	ExecutionFailed          ExecutionState = "failed"
	ExecutionCancelled       ExecutionState = "cancelled"
	ExecutionInterrupted     ExecutionState = "interrupted"
)

func ExecutionStates() []ExecutionState {
	return []ExecutionState{
		ExecutionQueued, ExecutionLeased, ExecutionPreparing, ExecutionRunning,
		ExecutionWaitingForInput, ExecutionQueuedForResume, ExecutionFinalizing,
		ExecutionAwaitingReview, ExecutionApproved, ExecutionCompleted, ExecutionFailed,
		ExecutionCancelled, ExecutionInterrupted,
	}
}

func TerminalExecutionStates() []ExecutionState {
	return []ExecutionState{
		ExecutionCompleted, ExecutionFailed, ExecutionCancelled, ExecutionInterrupted,
	}
}

func RunnerDrivenExecutionStates() []ExecutionState {
	return []ExecutionState{
		ExecutionPreparing, ExecutionRunning, ExecutionWaitingForInput, ExecutionFinalizing,
		ExecutionAwaitingReview, ExecutionCompleted, ExecutionFailed,
	}
}

func (s ExecutionState) Valid() bool {
	return slices.Contains(ExecutionStates(), s)
}

func (s ExecutionState) Terminal() bool {
	return slices.Contains(TerminalExecutionStates(), s)
}

func (s ExecutionState) RunnerDriven() bool {
	return slices.Contains(RunnerDrivenExecutionStates(), s)
}

func (s ExecutionState) Parked() bool {
	return s == ExecutionWaitingForInput || s == ExecutionAwaitingReview
}

func (s ExecutionState) HoldsLease() bool {
	return s != ExecutionQueued && !s.Terminal()
}

func (s ExecutionState) CanTransitionTo(target ExecutionState) bool {
	if !s.Valid() || !target.Valid() {
		return false
	}

	switch s {
	case ExecutionQueued:
		return target == ExecutionLeased || target == ExecutionCancelled || target == ExecutionFailed
	case ExecutionLeased:
		return target == ExecutionPreparing || abandons(target)
	case ExecutionPreparing:
		return target == ExecutionRunning || abandons(target)
	case ExecutionRunning:
		return target == ExecutionFinalizing || target == ExecutionWaitingForInput || abandons(target)
	case ExecutionWaitingForInput:
		return target == ExecutionQueuedForResume || abandons(target)
	case ExecutionQueuedForResume:
		return target == ExecutionRunning || abandons(target)
	case ExecutionFinalizing:
		return target == ExecutionAwaitingReview || target == ExecutionRunning || abandons(target)
	case ExecutionAwaitingReview:
		return target == ExecutionApproved || target == ExecutionQueuedForResume ||
			target == ExecutionCancelled || target == ExecutionFailed
	case ExecutionApproved:
		return target == ExecutionCompleted || target == ExecutionFailed
	default:
		return false
	}
}

func (s ExecutionState) MovesTheIssue() bool {
	return s == ExecutionRunning || s == ExecutionAwaitingReview || s == ExecutionCompleted
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

func abandons(target ExecutionState) bool {
	return target == ExecutionFailed || target == ExecutionCancelled ||
		target == ExecutionInterrupted
}

type ExecutionParams struct {
	Tool    string
	Model   string
	Runtime CodebaseRuntime
	Brief   string
}

type Execution struct {
	ID             string
	WorkspaceID    uuid.UUID
	IssueID        uuid.UUID
	IssueReference string
	TeamID         uuid.UUID
	DelegationID   uuid.UUID
	AgentID        uuid.UUID
	AgentName      string
	RunnerID       uuid.UUID
	CodebaseID     uuid.UUID
	Attempt        int
	State          ExecutionState
	Reason         string
	Params         ExecutionParams
	LeaseExpiresAt *time.Time
	QueuedAt       time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	UpdatedAt      time.Time
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

type ExecutionEventKind string

const (
	ExecutionEventTransition ExecutionEventKind = "transition"
	ExecutionEventPhase      ExecutionEventKind = "phase"
	ExecutionEventCommand    ExecutionEventKind = "command"
	ExecutionEventTool       ExecutionEventKind = "tool"
	ExecutionEventService    ExecutionEventKind = "service"
	ExecutionEventPreview    ExecutionEventKind = "preview"
	ExecutionEventNote       ExecutionEventKind = "note"
)

func ExecutionEventKinds() []ExecutionEventKind {
	return []ExecutionEventKind{
		ExecutionEventTransition, ExecutionEventPhase, ExecutionEventCommand, ExecutionEventTool,
		ExecutionEventService, ExecutionEventPreview, ExecutionEventNote,
	}
}

func (k ExecutionEventKind) Valid() bool {
	return slices.Contains(ExecutionEventKinds(), k)
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

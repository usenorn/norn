package channelv1

import "slices"

type State string

const (
	StateQueued          State = "queued"
	StateLeased          State = "leased"
	StatePreparing       State = "preparing"
	StateRunning         State = "running"
	StateWaitingForInput State = "waiting_for_input"
	StateQueuedForResume State = "queued_for_resume"
	StateFinalizing      State = "finalizing"
	StateAwaitingReview  State = "awaiting_review"
	StateApproved        State = "approved"
	StateCompleted       State = "completed"
	StateFailed          State = "failed"
	StateCancelled       State = "cancelled"
	StateInterrupted     State = "interrupted"
)

func States() []State {
	return []State{
		StateQueued, StateLeased, StatePreparing, StateRunning, StateWaitingForInput,
		StateQueuedForResume, StateFinalizing, StateAwaitingReview, StateApproved, StateCompleted,
		StateFailed, StateCancelled, StateInterrupted,
	}
}

func TerminalStates() []State {
	return []State{StateCompleted, StateFailed, StateCancelled, StateInterrupted}
}

func RunnerDrivenStates() []State {
	return []State{
		StatePreparing, StateRunning, StateWaitingForInput, StateFinalizing, StateAwaitingReview,
		StateCompleted, StateFailed,
	}
}

func (s State) Valid() bool {
	return slices.Contains(States(), s)
}

func (s State) Terminal() bool {
	return slices.Contains(TerminalStates(), s)
}

func (s State) RunnerDriven() bool {
	return slices.Contains(RunnerDrivenStates(), s)
}

func (s State) Parked() bool {
	return s == StateWaitingForInput || s == StateAwaitingReview
}

func (s State) HoldsLease() bool {
	return s != StateQueued && !s.Terminal()
}

func (s State) HoldsSlot() bool {
	return s == StatePreparing || s == StateRunning || s == StateFinalizing
}

func (s State) CanTransitionTo(target State) bool {
	if !s.Valid() || !target.Valid() {
		return false
	}

	switch s {
	case StateQueued:
		return target == StateLeased || target == StateCancelled || target == StateFailed
	case StateLeased:
		return target == StatePreparing || abandons(target)
	case StatePreparing:
		return target == StateRunning || abandons(target)
	case StateRunning:
		return target == StateFinalizing || target == StateWaitingForInput || abandons(target)
	case StateWaitingForInput:
		return target == StateQueuedForResume || abandons(target)
	case StateQueuedForResume:
		return target == StateRunning || abandons(target)
	case StateFinalizing:
		return target == StateAwaitingReview || target == StateRunning || abandons(target)
	case StateAwaitingReview:
		return target == StateApproved || target == StateQueuedForResume ||
			target == StateCancelled || target == StateFailed
	case StateApproved:
		return target == StateCompleted || target == StateFailed
	default:
		return false
	}
}

func abandons(target State) bool {
	return target == StateFailed || target == StateCancelled || target == StateInterrupted
}

type EventKind string

const (
	EventTransition EventKind = "transition"
	EventPhase      EventKind = "phase"
	EventCommand    EventKind = "command"
	EventTool       EventKind = "tool"
	EventService    EventKind = "service"
	EventPreview    EventKind = "preview"
	EventQuestion   EventKind = "question"
	EventNote       EventKind = "note"
)

func EventKinds() []EventKind {
	return []EventKind{
		EventTransition, EventPhase, EventCommand, EventTool, EventService, EventPreview,
		EventQuestion, EventNote,
	}
}

func (k EventKind) Valid() bool {
	return slices.Contains(EventKinds(), k)
}

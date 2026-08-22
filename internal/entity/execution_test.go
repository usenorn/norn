package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func permittedExecutionMoves() map[entity.ExecutionState][]entity.ExecutionState {
	return map[entity.ExecutionState][]entity.ExecutionState{
		entity.ExecutionQueued: {
			entity.ExecutionLeased, entity.ExecutionCancelled, entity.ExecutionFailed,
		},
		entity.ExecutionLeased: {
			entity.ExecutionPreparing, entity.ExecutionFailed, entity.ExecutionCancelled,
			entity.ExecutionInterrupted,
		},
		entity.ExecutionPreparing: {
			entity.ExecutionRunning, entity.ExecutionFailed, entity.ExecutionCancelled,
			entity.ExecutionInterrupted,
		},
		entity.ExecutionRunning: {
			entity.ExecutionFinalizing, entity.ExecutionWaitingForInput, entity.ExecutionFailed,
			entity.ExecutionCancelled, entity.ExecutionInterrupted,
		},
		entity.ExecutionWaitingForInput: {
			entity.ExecutionQueuedForResume, entity.ExecutionFailed, entity.ExecutionCancelled,
			entity.ExecutionInterrupted,
		},
		entity.ExecutionQueuedForResume: {
			entity.ExecutionRunning, entity.ExecutionFailed, entity.ExecutionCancelled,
			entity.ExecutionInterrupted,
		},
		entity.ExecutionFinalizing: {
			entity.ExecutionAwaitingReview, entity.ExecutionRunning, entity.ExecutionFailed,
			entity.ExecutionCancelled, entity.ExecutionInterrupted,
		},
		entity.ExecutionAwaitingReview: {
			entity.ExecutionApproved, entity.ExecutionQueuedForResume, entity.ExecutionCancelled,
			entity.ExecutionFailed,
		},
		entity.ExecutionApproved: {
			entity.ExecutionCompleted, entity.ExecutionFailed,
		},
	}
}

func TestEveryOrderedPairOfExecutionStatesIsDecidedDeliberately(t *testing.T) {
	permitted := permittedExecutionMoves()

	for _, from := range entity.ExecutionStates() {
		for _, to := range entity.ExecutionStates() {
			want := false

			for _, allowed := range permitted[from] {
				if allowed == to {
					want = true
				}
			}

			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s -> %s is %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestNoExecutionStateTransitionsToItself(t *testing.T) {
	for _, state := range entity.ExecutionStates() {
		if state.CanTransitionTo(state) {
			t.Errorf(
				"%s -> %s is permitted; a runner repeating its last state would append a second "+
					"transition to the timeline and restamp when the execution entered it",
				state, state,
			)
		}
	}
}

func TestAFinishedExecutionNeverMovesAgain(t *testing.T) {
	for _, terminal := range entity.TerminalExecutionStates() {
		for _, target := range entity.ExecutionStates() {
			if terminal.CanTransitionTo(target) {
				t.Errorf(
					"%s -> %s is permitted; re-running a finished execution is a new attempt, "+
						"not a move, because the old run's timeline has to stay readable",
					terminal, target,
				)
			}
		}
	}
}

func TestAFailedFinalizeGoesBackToRunningSoUncommittedWorkIsNotLost(t *testing.T) {
	if !entity.ExecutionFinalizing.CanTransitionTo(entity.ExecutionRunning) {
		t.Fatal(
			"finalizing cannot return to running; a finalize that finds an uncommitted tree has " +
				"nowhere to send the agent back to",
		)
	}
}

func TestOnlyTheThirteenKnownExecutionStatesAreValid(t *testing.T) {
	for _, state := range entity.ExecutionStates() {
		if !state.Valid() {
			t.Errorf("%q is listed as an execution state but does not validate", state)
		}
	}

	for _, state := range []entity.ExecutionState{"", "queued_for_review", "Running", "done"} {
		if state.Valid() {
			t.Errorf("%q validates as an execution state and should not", state)
		}
	}
}

func TestARunnerMayNotClaimAStateTheServerOwns(t *testing.T) {
	serverOwned := []entity.ExecutionState{
		entity.ExecutionQueued, entity.ExecutionLeased, entity.ExecutionQueuedForResume,
		entity.ExecutionApproved, entity.ExecutionCancelled, entity.ExecutionInterrupted,
	}

	for _, state := range serverOwned {
		if state.RunnerDriven() {
			t.Errorf(
				"a runner may report %q; only the server knows when an execution enters it",
				state,
			)
		}
	}

	for _, state := range []entity.ExecutionState{
		entity.ExecutionPreparing, entity.ExecutionRunning, entity.ExecutionFinalizing,
	} {
		if !state.RunnerDriven() {
			t.Errorf("a runner may not report %q, but it is the only party that knows", state)
		}
	}
}

func TestAParkedExecutionKeepsItsLeaseWithoutOccupyingASlot(t *testing.T) {
	for _, state := range []entity.ExecutionState{
		entity.ExecutionWaitingForInput, entity.ExecutionAwaitingReview,
	} {
		if !state.Parked() {
			t.Errorf("%q is not parked; its slot stays occupied while nobody is working", state)
		}

		if !state.HoldsLease() {
			t.Errorf("%q does not hold its lease; the sweep would interrupt a healthy run", state)
		}
	}

	if entity.ExecutionQueued.HoldsLease() {
		t.Error("a queued execution holds a lease before any runner has accepted it")
	}

	for _, terminal := range entity.TerminalExecutionStates() {
		if terminal.HoldsLease() {
			t.Errorf("%q still holds a lease after finishing", terminal)
		}
	}
}

func TestARerunIsNamedAfterItsIssueAndItsAttempt(t *testing.T) {
	cases := map[string]struct {
		attempt int
		want    string
	}{
		"the first run is just the issue": {1, "NORN-34"},
		"the second run is suffixed":      {2, "NORN-34-r2"},
		"the tenth run keeps counting":    {10, "NORN-34-r10"},
	}

	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			execution := entity.Execution{IssueReference: "NORN-34", Attempt: expected.attempt}

			if got := execution.Reference(); got != expected.want {
				t.Fatalf("attempt %d is named %q, want %q", expected.attempt, got, expected.want)
			}
		})
	}
}

func TestACompletedExecutionIsNotOfferedARestart(t *testing.T) {
	cases := map[entity.ExecutionState]bool{
		entity.ExecutionCompleted:   false,
		entity.ExecutionFailed:      true,
		entity.ExecutionCancelled:   true,
		entity.ExecutionInterrupted: true,
		entity.ExecutionRunning:     false,
	}

	for state, want := range cases {
		execution := entity.Execution{State: state}

		if got := execution.Restartable(); got != want {
			t.Errorf("a %q execution reports restartable=%v, want %v", state, got, want)
		}
	}
}

func TestOnlyTheThreeStatesThatMoveAnIssueResolveATargetState(t *testing.T) {
	states := []entity.WorkflowState{
		{Name: "Todo", Category: entity.StateCategoryNotStarted, Position: 1},
		{Name: "In progress", Category: entity.StateCategoryActive, Position: 2},
		{Name: "In review", Category: entity.StateCategoryActive, Position: 3},
		{Name: "Done", Category: entity.StateCategoryComplete, Position: 4, IsCompletion: true},
	}

	for _, state := range entity.ExecutionStates() {
		target, resolved := entity.IssueStateFor(state, states)

		if resolved != entity.MovesTheIssue(state) {
			t.Errorf(
				"%s resolves a target state=%v but reports MovesTheIssue=%v; the two disagree, so "+
					"either an issue moves without warning or a move is silently skipped",
				state, resolved, entity.MovesTheIssue(state),
			)
		}

		if !resolved {
			continue
		}

		wanted := map[entity.ExecutionState]string{
			entity.ExecutionRunning:        "In progress",
			entity.ExecutionAwaitingReview: "In review",
			entity.ExecutionCompleted:      "Done",
		}[state]

		if target.Name != wanted {
			t.Errorf("a %s run puts its issue in %q, want %q", state, target.Name, wanted)
		}
	}
}

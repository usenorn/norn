package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAMachineIsOnlyAvailableWhenEveryReasonToSkipItIsAbsent(t *testing.T) {
	free := entity.RunnerLoad{Capacity: 2, Used: 1}

	cases := []struct {
		name      string
		presence  entity.RunnerPresence
		available bool
	}{
		{
			name:      "connected with a slot spare",
			presence:  entity.RunnerPresence{Epoch: "live", Load: free},
			available: true,
		},
		{
			name:     "no channel at all",
			presence: entity.RunnerPresence{Load: free},
		},
		{
			name: "every slot in use",
			presence: entity.RunnerPresence{
				Epoch: "live",
				Load:  entity.RunnerLoad{Capacity: 2, Used: 2},
			},
		},
		{
			name: "paused by whoever is at the keyboard",
			presence: entity.RunnerPresence{
				Epoch: "live",
				Load:  entity.RunnerLoad{Capacity: 2, Used: 0, Paused: true},
			},
		},
		{
			name: "nearly out of disk",
			presence: entity.RunnerPresence{
				Epoch: "live",
				Load:  entity.RunnerLoad{Capacity: 2, Used: 0, DiskPressure: true},
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := test.presence.Available(); got != test.available {
				t.Fatalf(
					"a machine %s reads available=%v, want %v; work offered to a machine that "+
						"cannot take it comes straight back as a decline and the run waits a "+
						"whole heartbeat longer than it had to",
					test.name, got, test.available,
				)
			}
		})
	}
}

func TestAMachineReportingMoreInUseThanItHasIsSaidToHaveNoRoom(t *testing.T) {
	crowded := entity.RunnerPresence{
		Epoch: "live",
		Load:  entity.RunnerLoad{Capacity: 2, Used: 5},
	}

	if got := crowded.Free(); got != 0 {
		t.Fatalf(
			"a machine holding more runs than its capacity reports %d slots free, want 0. A "+
				"negative count read as room would keep handing work to the busiest machine "+
				"there is",
			got,
		)
	}
}

func TestCountingSlotsFollowsWhatTheMachineIsActuallyDoing(t *testing.T) {
	holding := map[entity.ExecutionState]bool{
		entity.ExecutionQueued:          false,
		entity.ExecutionLeased:          false,
		entity.ExecutionPreparing:       true,
		entity.ExecutionRunning:         true,
		entity.ExecutionWaitingForInput: false,
		entity.ExecutionQueuedForResume: false,
		entity.ExecutionFinalizing:      true,
		entity.ExecutionAwaitingReview:  false,
		entity.ExecutionCompleted:       false,
	}

	for state, occupies := range holding {
		if got := state.HoldsSlot(); got != occupies {
			t.Fatalf(
				"%s holds a slot=%v, want %v. A run parked on a question has no coding agent "+
					"running, so counting it against the machine costs somebody a slot for as "+
					"long as nobody answers",
				state, got, occupies,
			)
		}
	}
}

func TestARuntimeOverrideOfAutoAsksTheMachineForNothing(t *testing.T) {
	if got := entity.RuntimeChoiceAuto.Override(); got != "" {
		t.Fatalf(
			"auto reached the machine as %q; the machine resolves the runtime per run, and "+
				"sending it a value it has to interpret as absence invites the two to disagree",
			got,
		)
	}

	if got := entity.RuntimeChoiceDocker.Override(); got != entity.CodebaseRuntimeDocker {
		t.Fatalf("asking for docker reached the machine as %q", got)
	}
}

func TestADelegationMayLeaveEveryParameterToTheMachine(t *testing.T) {
	if fields := entity.NewValidationError(
		entity.ValidateDelegationParams("params", entity.DelegationParams{})...,
	); fields != nil {
		t.Fatalf(
			"delegating with nothing chosen was refused with %v; the common path is one click "+
				"and the machine's own settings are what stand",
			fields,
		)
	}
}

func TestADelegationCannotAskForAProfileThatDoesNotExist(t *testing.T) {
	fields := entity.ValidateDelegationParams("params", entity.DelegationParams{
		Profile: "anything-goes",
	})

	if err := entity.NewValidationError(fields...); err == nil {
		t.Fatal(
			"a permission profile nobody defined was accepted. It reaches the machine, which " +
				"falls back to its own default, so the run is quietly more permissive than the " +
				"person asked for",
		)
	}
}

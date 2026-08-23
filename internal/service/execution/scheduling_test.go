package execution_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (h *harness) machine(name string) entity.Runner {
	return entity.Runner{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AgentID:     h.runner.AgentID,
		Name:        name,
		Authority:   entity.RequestedAuthority{AllTeams: true},
		Status:      entity.RunnerStatusActive,
	}
}

func (h *harness) delegation() entity.IssueDelegation {
	return entity.IssueDelegation{
		ID:      uuid.New(),
		IssueID: h.issue.ID,
		AgentID: h.runner.AgentID,
		Brief:   "carry the slice",
	}
}

func TestTheOfferGoesToTheMachineWithTheMostRoomForIt(t *testing.T) {
	h := newHarness(t)

	busy := h.machine("busy")
	idle := h.machine("idle")

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{busy, idle}, nil)

	h.live(busy, 4, 3)
	h.live(idle, 4, 1)

	created := h.opening(1)

	if err := h.service.OnDelegated(context.Background(), h.issue, h.delegation()); err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if created.RunnerID != idle.ID {
		t.Fatalf(
			"the run went to the machine with one slot free rather than the one with three; " +
				"work piles onto whichever machine happens to be listed first and the rest of " +
				"the fleet sits idle",
		)
	}

	if len(h.spooled) != 1 {
		t.Fatalf(
			"%d offers went out for one delegation, want exactly one; two machines that both "+
				"hold the codebase must not both start it",
			len(h.spooled),
		)
	}
}

func TestAFullMachineLeavesTheWorkQueuedAndSaysWhy(t *testing.T) {
	h := newHarness(t)

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner}, nil)

	h.live(h.runner, 2, 2)

	created := h.opening(1)

	if err := h.service.OnDelegated(context.Background(), h.issue, h.delegation()); err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if len(h.spooled) != 0 {
		t.Fatalf("work was offered to a machine with no free slot: %+v", h.spooled)
	}

	if created.QueuedReason != entity.QueuedRunnersBusy {
		t.Fatalf(
			"a run waiting on a full machine is waiting for %q, want %q; the dialog says "+
				"something different for busy than for offline and cannot tell them apart "+
				"without this",
			created.QueuedReason, entity.QueuedRunnersBusy,
		)
	}
}

func TestAFreedSlotStartsTheWaitingRunOnTheNextHeartbeat(t *testing.T) {
	h := newHarness(t)

	waiting := h.execution(entity.ExecutionQueued)
	waiting.RunnerID = uuid.Nil
	waiting.QueuedReason = entity.QueuedRunnersBusy

	h.live(h.runner, 2, 1)
	h.binding()

	h.executions.EXPECT().
		ListQueuedByAgent(gomock.Any(), h.runner.AgentID, gomock.Any()).
		Return([]entity.Execution{waiting}, nil)

	if err := h.service.Ready(context.Background(), h.runner); err != nil {
		t.Fatalf("a heartbeat reporting a free slot failed: %v", err)
	}

	offer, sent := h.sent(entity.ChannelExecutionOffer)
	if !sent {
		t.Fatal(
			"a machine reported a free slot and the run it was waiting on stayed queued. " +
				"Nothing else polls, so the work waits until somebody delegates again",
		)
	}

	if offer.ExecutionID != waiting.ID {
		t.Fatalf("the offer names %q, want the run that was waiting", offer.ExecutionID)
	}

	if len(h.bound) != 1 || h.bound[0].RunnerID != h.runner.ID {
		t.Fatalf("the waiting run was not bound to the machine that took it: %+v", h.bound)
	}
}

func TestAPausedMachineIsOfferedNothing(t *testing.T) {
	h := newHarness(t)

	paused := time.Now().UTC()
	standing := h.runner
	standing.PausedAt = &paused

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{standing}, nil)

	h.live(standing, 4, 0)

	created := h.opening(1)

	if err := h.service.OnDelegated(context.Background(), h.issue, h.delegation()); err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if len(h.spooled) != 0 {
		t.Fatalf(
			"a paused machine with three free slots was offered work: %+v. Pausing is how "+
				"somebody takes a machine out of rotation before touching it",
			h.spooled,
		)
	}

	if created.QueuedReason != entity.QueuedRunnersPaused {
		t.Fatalf(
			"the run is waiting for %q, want %q",
			created.QueuedReason, entity.QueuedRunnersPaused,
		)
	}
}

func TestAPausedMachineIsStillAskedForNothingOnItsOwnHeartbeat(t *testing.T) {
	h := newHarness(t)

	paused := time.Now().UTC()
	standing := h.runner
	standing.PausedAt = &paused

	h.live(standing, 4, 0)

	if err := h.service.Ready(context.Background(), standing); err != nil {
		t.Fatalf("heartbeat from a paused machine: %v", err)
	}

	if len(h.spooled) != 0 {
		t.Fatalf("a paused machine was handed work by its own heartbeat: %+v", h.spooled)
	}
}

func TestAMachineThatTurnsWorkDownDoesNotGetItBackTheSameMoment(t *testing.T) {
	h := newHarness(t)

	offered := h.execution(entity.ExecutionQueued)
	offered.RunnerID = h.runner.ID

	h.holding(offered)
	h.binding()

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner}, nil)

	if err := h.service.Declined(context.Background(), h.runner, message(
		"01DEC", entity.ChannelExecutionDeclined, offered.ID,
		channelv1.Decline{Code: channelv1.DeclineAtCapacity, Detail: "2 of 2 in use"},
	)); err != nil {
		t.Fatalf("decline: %v", err)
	}

	if len(h.spooled) != 0 {
		t.Fatalf(
			"the machine that just said it was full was offered the same run again: %+v. That "+
				"is a loop between two processes, and the heartbeat is what should move it",
			h.spooled,
		)
	}

	if len(h.bound) != 1 {
		t.Fatalf("a declined run was not put back on the queue: %+v", h.bound)
	}

	if h.bound[0].RunnerID != uuid.Nil {
		t.Fatalf(
			"a declined run is still bound to %v; nothing would ever offer it elsewhere and the "+
				"lease sweep does not look at queued runs, so it would sit there for good",
			h.bound[0].RunnerID,
		)
	}

	if h.bound[0].QueuedReason != entity.QueuedRunnersBusy {
		t.Fatalf(
			"a run declined for capacity is waiting for %q, want %q",
			h.bound[0].QueuedReason, entity.QueuedRunnersBusy,
		)
	}
}

func TestWorkTurnedDownByOneMachineGoesToAnother(t *testing.T) {
	h := newHarness(t)

	spare := h.machine("spare")

	offered := h.execution(entity.ExecutionQueued)
	offered.RunnerID = h.runner.ID

	h.holding(offered)
	h.binding()

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner, spare}, nil)

	h.live(spare, 2, 0)

	if err := h.service.Declined(context.Background(), h.runner, message(
		"01DEC", entity.ChannelExecutionDeclined, offered.ID,
		channelv1.Decline{Code: channelv1.DeclineDiskPressure, Detail: "3GB left"},
	)); err != nil {
		t.Fatalf("decline: %v", err)
	}

	if _, sent := h.sent(entity.ChannelExecutionOffer); !sent {
		t.Fatal(
			"one machine turned the work down and the agent's other machine, idle and " +
				"connected, was never asked",
		)
	}

	if h.bound[len(h.bound)-1].RunnerID != spare.ID {
		t.Fatalf("the work was not handed to the idle machine: %+v", h.bound)
	}
}

func TestTheReasonAMachineTurnedWorkDownStaysOnTheTimeline(t *testing.T) {
	h := newHarness(t)

	offered := h.execution(entity.ExecutionQueued)
	offered.RunnerID = h.runner.ID

	h.holding(offered)
	h.binding()

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner}, nil)

	if err := h.service.Declined(context.Background(), h.runner, message(
		"01DEC", entity.ChannelExecutionDeclined, offered.ID,
		channelv1.Decline{
			Code:   channelv1.DeclineDiskPressure,
			Detail: "3GB free, below the 10GB this machine keeps back",
		},
	)); err != nil {
		t.Fatalf("decline: %v", err)
	}

	note, recorded := h.entry(entity.ExecutionEventNote)
	if !recorded {
		t.Fatal("nothing on the timeline says the machine turned the work down")
	}

	if note.Reason != "disk_pressure: 3GB free, below the 10GB this machine keeps back" {
		t.Fatalf(
			"the timeline reads %q; somebody reading it has to see both which rule the machine "+
				"applied and the figures it applied it to",
			note.Reason,
		)
	}
}

func TestAnOfflineMachineLeavesTheWorkQueuedAndSaysWhy(t *testing.T) {
	h := newHarness(t)

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner}, nil)

	h.offline(h.runner)

	created := h.opening(1)

	if err := h.service.OnDelegated(context.Background(), h.issue, h.delegation()); err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if len(h.spooled) != 0 {
		t.Fatalf(
			"an offer was spooled at a machine that is not connected: %+v. It would sit in the "+
				"spool while a second, connected machine of the same agent stayed idle",
			h.spooled,
		)
	}

	if created.QueuedReason != entity.QueuedRunnersOffline {
		t.Fatalf(
			"a run waiting on a switched-off machine is waiting for %q, want %q; offline is "+
				"somebody's laptop being shut, and busy is not, so the two cannot read the same",
			created.QueuedReason, entity.QueuedRunnersOffline,
		)
	}
}

func TestABusyMachineOutranksAnOfflineOneWhenSayingWhyTheWorkWaits(t *testing.T) {
	h := newHarness(t)

	away := h.machine("away")
	full := h.machine("full")

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{away, full}, nil)

	h.offline(away)
	h.live(full, 1, 1)

	created := h.opening(1)

	if err := h.service.OnDelegated(context.Background(), h.issue, h.delegation()); err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if created.QueuedReason != entity.QueuedRunnersBusy {
		t.Fatalf(
			"with one machine away and one working flat out the run reads %q, want %q. Telling "+
				"somebody their machines are offline when one is right there working sends them "+
				"to check a connection that is fine",
			created.QueuedReason, entity.QueuedRunnersBusy,
		)
	}
}

func TestAnotherRunOnTheSameRepositoryIsWorthSayingBeforeDelegating(t *testing.T) {
	h := newHarness(t)

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner}, nil).
		Times(2)

	h.live(h.runner, 2, 0)

	elsewhere := h.execution(entity.ExecutionRunning)
	elsewhere.ID = "exec-01OTHER"

	h.executions.EXPECT().
		ListSharingRepositories(gomock.Any(), h.workspaceID, "", h.codebase, gomock.Any()).
		Return([]entity.Execution{elsewhere}, nil)

	placement, err := h.service.Placement(context.Background(), h.issue, h.runner.AgentID)
	if err != nil {
		t.Fatalf("placement: %v", err)
	}

	if len(placement.Sharing) != 1 || placement.Sharing[0].ID != elsewhere.ID {
		t.Fatalf(
			"delegating did not mention the run already going in the same repository: %+v. Two "+
				"agents editing one checkout is worth knowing before starting, even though "+
				"snapshots keep them apart",
			placement.Sharing,
		)
	}

	if placement.RunnerID != h.runner.ID {
		t.Fatalf(
			"a warning turned into a refusal: the work would go nowhere. It is a warning, never " +
				"a block",
		)
	}
}

func TestPlacementSaysWhichMachinesAreThereAndHowMuchRoomTheyHave(t *testing.T) {
	h := newHarness(t)

	away := h.machine("away")

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner, away}, nil).
		Times(2)

	h.live(h.runner, 4, 1)
	h.offline(away)

	h.executions.EXPECT().
		ListSharingRepositories(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil)

	placement, err := h.service.Placement(context.Background(), h.issue, h.runner.AgentID)
	if err != nil {
		t.Fatalf("placement: %v", err)
	}

	if len(placement.Runners) != 2 {
		t.Fatalf("the dialog was told about %d machines, want both", len(placement.Runners))
	}

	if !placement.Runners[0].Connected || placement.Runners[0].Free != 3 {
		t.Fatalf(
			"the connected machine reads connected=%v free=%d, want true and 3; the dialog says "+
				"where the work will run and how much room is left there",
			placement.Runners[0].Connected, placement.Runners[0].Free,
		)
	}

	if placement.Runners[1].Connected {
		t.Fatal("a machine with no channel was reported as connected")
	}
}

func TestAnOfferAMachineNeverAnsweredIsPutBackInFrontOfIt(t *testing.T) {
	h := newHarness(t)

	unanswered := h.execution(entity.ExecutionQueued)

	h.live(h.runner, 2, 0)
	h.binding()

	h.executions.EXPECT().
		ListQueuedByAgent(gomock.Any(), h.runner.AgentID, gomock.Any()).
		Return([]entity.Execution{unanswered}, nil)

	if err := h.service.Ready(context.Background(), h.runner); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	if _, sent := h.sent(entity.ChannelExecutionOffer); !sent {
		t.Fatal(
			"a machine that was offered work and died before saying yes or no was never asked " +
				"again. The run holds no lease while it is queued, so the sweep never sees it " +
				"and it waits for somebody to notice by hand",
		)
	}
}

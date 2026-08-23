package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func TestDelegatingAnIssueOffersTheWorkToTheAgentsMachine(t *testing.T) {
	h := newHarness(t)

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner}, nil)
	h.live(h.runner, 2, 0)

	created := h.opening(1)

	delegation := entity.IssueDelegation{
		ID:      uuid.New(),
		IssueID: h.issue.ID,
		AgentID: h.runner.AgentID,
		Brief:   "ship the state machine",
	}

	if err := h.service.OnDelegated(context.Background(), h.issue, delegation); err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if created.RunnerID != h.runner.ID {
		t.Fatalf("the run was opened bound to %v, want the machine that was offered it", created.RunnerID)
	}

	if created.Params.Brief != delegation.Brief {
		t.Fatalf("the offer carries brief %q, want %q", created.Params.Brief, delegation.Brief)
	}

	offer, sent := h.sent(entity.ChannelExecutionOffer)
	if !sent {
		t.Fatal("nothing was offered to the machine, so the delegation would sit forever")
	}

	if offer.ExecutionID != created.ID {
		t.Fatalf("the offer names %q, want %q", offer.ExecutionID, created.ID)
	}
}

func TestWorkOnlyGoesToAMachineAllowedToSeeTheTeamItBelongsTo(t *testing.T) {
	h := newHarness(t)

	elsewhere := h.runner
	elsewhere.Authority = entity.RequestedAuthority{TeamIDs: []uuid.UUID{uuid.New()}}

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{elsewhere}, nil)

	created := h.opening(1)

	err := h.service.OnDelegated(context.Background(), h.issue, entity.IssueDelegation{
		ID:      uuid.New(),
		IssueID: h.issue.ID,
		AgentID: h.runner.AgentID,
		Brief:   "the brief is the leak",
	})
	if err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if len(h.spooled) != 0 {
		t.Fatalf(
			"an issue was offered to a machine enrolled for another team: %+v. The offer carries "+
				"the issue title, description and brief, so routing past a runner's team scope "+
				"hands that content to a machine the token was never scoped to reach",
			h.spooled,
		)
	}

	if created.QueuedReason != entity.QueuedNoRunner {
		t.Fatalf(
			"a run nobody in scope can take is waiting for %q, want %q; the dialog has to say "+
				"this agent has no machine that reaches the team rather than showing a spinner",
			created.QueuedReason, entity.QueuedNoRunner,
		)
	}
}

func TestAnAgentWithNoMachineQueuesTheRunInsteadOfLosingIt(t *testing.T) {
	h := newHarness(t)

	h.runners.EXPECT().ListByAgentID(gomock.Any(), h.runner.AgentID).Return(nil, nil)

	created := h.opening(1)

	err := h.service.OnDelegated(context.Background(), h.issue, entity.IssueDelegation{
		ID:      uuid.New(),
		IssueID: h.issue.ID,
		AgentID: h.runner.AgentID,
	})
	if err != nil {
		t.Fatalf(
			"delegating to an agent with no runner failed with %v; an agent that only answers "+
				"over MCP must still be delegatable",
			err,
		)
	}

	if len(h.spooled) != 0 {
		t.Fatalf("an offer was spooled for a machine that does not exist: %+v", h.spooled)
	}

	if created.RunnerID != uuid.Nil {
		t.Fatalf("the run was bound to %v, and no machine exists to bind it to", created.RunnerID)
	}

	if created.QueuedReason != entity.QueuedNoRunner {
		t.Fatalf(
			"the run is waiting for %q, want %q; without a run there is nothing for the issue "+
				"screen to show and nothing for a machine to pick up when one is connected",
			created.QueuedReason, entity.QueuedNoRunner,
		)
	}
}

func TestAcceptingAnOfferLeasesTheRunAndTellsTheMachineToBegin(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionQueued)
	h.holding(execution)
	h.moving()

	err := h.service.Accepted(context.Background(), h.runner, message(
		"01ACC", entity.ChannelExecutionAccepted, execution.ID, nil,
	))
	if err != nil {
		t.Fatalf("accept an offer: %v", err)
	}

	start, sent := h.sent(entity.ChannelExecutionStart)
	if !sent {
		t.Fatal("the machine accepted an offer and was never told to start")
	}

	if start.ExecutionID != execution.ID {
		t.Fatalf("the start names %q, want %q", start.ExecutionID, execution.ID)
	}
}

func TestARunnerMayNotReportAStateOnlyTheServerDecides(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	h.holding(execution)

	err := h.service.Reported(context.Background(), h.runner, message(
		"01SRV", entity.ChannelExecutionState, execution.ID,
		map[string]string{"state": string(entity.ExecutionApproved)},
	))

	if !errors.Is(err, entity.ErrExecutionStateNotRunners) {
		t.Fatalf(
			"a machine approved its own work and got %v; approval is the reviewer's, not the "+
				"runner's",
			err,
		)
	}
}

func TestARunnerCannotTouchAnotherMachinesRun(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	execution.RunnerID = uuid.New()
	h.holding(execution)

	err := h.service.Reported(context.Background(), h.runner, message(
		"01OTH", entity.ChannelExecutionState, execution.ID,
		map[string]string{"state": string(entity.ExecutionFinalizing)},
	))

	if !errors.Is(err, entity.ErrExecutionNotFound) {
		t.Fatalf(
			"reporting on somebody else's run returned %v, want it read as not found rather "+
				"than forbidden, which would confirm the run exists",
			err,
		)
	}
}

func TestAnIllegalStateFromARunnerIsRefusedRatherThanRecorded(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionQueued)
	h.holding(execution)

	err := h.service.Reported(context.Background(), h.runner, message(
		"01ILL", entity.ChannelExecutionState, execution.ID,
		map[string]string{"state": string(entity.ExecutionFinalizing)},
	))

	if !errors.Is(err, entity.ErrExecutionTransition) {
		t.Fatalf("a queued run jumped straight to finalizing and got %v", err)
	}
}

func TestARunThatIsRunningPutsItsIssueInProgress(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionPreparing)
	h.holding(execution)
	h.moving()

	states := h.states34()
	h.states.EXPECT().ListByTeamID(gomock.Any(), h.issue.TeamID).Return(states, nil)

	var moved uuid.UUID

	h.writer.EXPECT().
		Update(gomock.Any(), h.workspaceID, h.issue.ID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ uuid.UUID, input service.UpdateIssueInput,
		) (entity.Issue, error) {
			moved = *input.StateID

			return h.issue, nil
		})

	err := h.service.Reported(context.Background(), h.runner, message(
		"01RUN", entity.ChannelExecutionState, execution.ID,
		map[string]string{"state": string(entity.ExecutionRunning)},
	))
	if err != nil {
		t.Fatalf("report running: %v", err)
	}

	if moved != states[1].ID {
		t.Fatalf("the issue moved to %v, want the first active state %q", moved, states[1].Name)
	}
}

func TestARunAwaitingReviewPutsItsIssueInReview(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionFinalizing)
	h.holding(execution)
	h.moving()

	states := h.states34()
	h.states.EXPECT().ListByTeamID(gomock.Any(), h.issue.TeamID).Return(states, nil)

	var moved uuid.UUID

	h.writer.EXPECT().
		Update(gomock.Any(), h.workspaceID, h.issue.ID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ uuid.UUID, input service.UpdateIssueInput,
		) (entity.Issue, error) {
			moved = *input.StateID

			return h.issue, nil
		})

	err := h.service.Reported(context.Background(), h.runner, message(
		"01REV", entity.ChannelExecutionState, execution.ID,
		map[string]string{"state": string(entity.ExecutionAwaitingReview)},
	))
	if err != nil {
		t.Fatalf("report awaiting review: %v", err)
	}

	if moved != states[2].ID {
		t.Fatalf("the issue moved to %v, want the last active state %q", moved, states[2].Name)
	}
}

func TestCancellingTellsTheMachineToStop(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	h.holding(execution)
	h.moving()
	h.states.EXPECT().ListByTeamID(gomock.Any(), gomock.Any()).Return(h.states34(), nil).AnyTimes()

	cancelled, err := h.service.Cancel(
		context.Background(), h.workspaceID, execution.ID, "changed my mind",
	)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if cancelled.State != entity.ExecutionCancelled {
		t.Fatalf("cancelling left the run %q", cancelled.State)
	}

	if _, sent := h.sent(entity.ChannelExecutionCancel); !sent {
		t.Fatal("the run was cancelled server-side and the machine was never told to tear it down")
	}
}

func TestAFinishedRunCannotBeCancelledAgain(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionCompleted)
	h.holding(execution)

	_, err := h.service.Cancel(context.Background(), h.workspaceID, execution.ID, "")
	if !errors.Is(err, entity.ErrExecutionFinished) {
		t.Fatalf("cancelling a completed run returned %v, want it refused", err)
	}
}

func TestOnlyARunWaitingForReviewCanBeApprovedOrSentBack(t *testing.T) {
	for _, state := range []entity.ExecutionState{
		entity.ExecutionRunning, entity.ExecutionQueued, entity.ExecutionCompleted,
	} {
		t.Run(string(state), func(t *testing.T) {
			h := newHarness(t)
			execution := h.execution(state)
			h.holding(execution)

			if _, err := h.service.Approve(
				context.Background(), h.workspaceID, execution.ID,
			); !errors.Is(err, entity.ErrExecutionNotReviewable) {
				t.Fatalf("approving a %q run returned %v", state, err)
			}

			if _, err := h.service.Resume(
				context.Background(), h.workspaceID, execution.ID, "try again",
			); !errors.Is(err, entity.ErrExecutionNotReviewable) {
				t.Fatalf("sending a %q run back returned %v", state, err)
			}
		})
	}
}

func TestAnAgentCannotApproveItsOwnWorkOverTheAPI(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionAwaitingReview)
	h.holding(execution)

	itself := execution.AgentID
	h.callerAgent = &itself

	_, err := h.service.Approve(context.Background(), h.workspaceID, execution.ID)
	if !errors.Is(err, entity.ErrExecutionSelfApproval) {
		t.Fatalf(
			"an agent approved its own execution and got %v; the channel already refuses a runner "+
				"claiming approved, so leaving the same move open over HTTP to a token carrying "+
				"issue:manage would make review optional for anything that runs the work",
			err,
		)
	}
}

func TestSendingARunBackHandsTheFeedbackToTheMachine(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionAwaitingReview)
	h.holding(execution)
	h.moving()
	h.states.EXPECT().ListByTeamID(gomock.Any(), gomock.Any()).Return(h.states34(), nil).AnyTimes()

	resumed, err := h.service.Resume(
		context.Background(), h.workspaceID, execution.ID, "the migration is missing a down",
	)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}

	if resumed.State != entity.ExecutionQueuedForResume {
		t.Fatalf("the run went to %q, want queued_for_resume", resumed.State)
	}

	sent, delivered := h.sent(entity.ChannelExecutionResume)
	if !delivered {
		t.Fatal("the reviewer's feedback never reached the machine")
	}

	if !strings.Contains(string(sent.Payload), "the migration is missing a down") {
		t.Fatalf("the resume carries %s, which does not repeat the feedback", sent.Payload)
	}
}

func TestRestartingOpensTheNextAttemptAndLeavesTheOldRunAlone(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionFailed)
	h.holding(execution)

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner}, nil)
	h.live(h.runner, 2, 0)

	h.opening(2)

	restarted, err := h.service.Restart(context.Background(), h.workspaceID, execution.ID)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}

	switch {
	case restarted.ID == execution.ID:
		t.Fatal("restarting reused the failed run's id, so its timeline would be overwritten")
	case restarted.Attempt != 2:
		t.Fatalf("the new run is attempt %d, want 2", restarted.Attempt)
	case restarted.Reference() != "NORN-34-r2":
		t.Fatalf("the new run is named %q, want NORN-34-r2", restarted.Reference())
	}
}

func TestARunStillGoingCannotBeRestarted(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	h.holding(execution)

	_, err := h.service.Restart(context.Background(), h.workspaceID, execution.ID)
	if !errors.Is(err, entity.ErrExecutionUnfinished) {
		t.Fatalf(
			"a running execution was restarted and got %v; two runs of one delegation must "+
				"never exist at once",
			err,
		)
	}
}

func TestATimelineEntryTheMachineSendsTwiceIsRecordedOnce(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	h.holding(execution)

	entry := message("01DUP", entity.ChannelExecutionEvent, execution.ID, map[string]string{
		"kind":   string(entity.ExecutionEventCommand),
		"reason": "go test ./...",
	})

	for range 2 {
		if err := h.service.Observed(context.Background(), h.runner, entry); err != nil {
			t.Fatalf("a redelivered timeline entry failed the connection: %v", err)
		}
	}

	if len(h.recorded) != 1 {
		t.Fatalf(
			"a message the machine sent twice landed on the timeline %d times; delivery is "+
				"at-least-once, so a reconnect would double every entry",
			len(h.recorded),
		)
	}

	if h.recorded[0].Kind != entity.ExecutionEventCommand {
		t.Fatalf("the entry was recorded as %q, want the kind the machine reported",
			h.recorded[0].Kind)
	}
}

func TestATimelineEntryWithAMalformedDetailDoesNotDropTheConnection(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	h.holding(execution)

	for name, detail := range map[string]any{
		"an array":  []int{1, 2},
		"a string":  "not an object",
		"a number":  7,
		"null":      nil,
		"an object": map[string]any{"exit": 0},
	} {
		t.Run(name, func(t *testing.T) {
			entry := message("01BAD"+name, entity.ChannelExecutionEvent, execution.ID, map[string]any{
				"kind": string(entity.ExecutionEventNote), "detail": detail,
			})

			if err := h.service.Observed(context.Background(), h.runner, entry); err != nil {
				t.Fatalf(
					"a timeline entry carrying %s failed with %v; the channel edge turns that into "+
						"a closed socket, so one malformed field would put a runner in a "+
						"reconnect loop",
					name, err,
				)
			}
		})
	}

	for _, recorded := range h.recorded {
		if len(recorded.Detail) == 0 {
			continue
		}

		var decoded map[string]any
		if err := json.Unmarshal(recorded.Detail, &decoded); err != nil || decoded == nil {
			t.Fatalf(
				"a detail of %s reached the timeline; the column only accepts a JSON object, so "+
					"the insert would be refused",
				recorded.Detail,
			)
		}
	}
}

func TestALapsedLeaseInterruptsTheRunAndLeavesItRestartable(t *testing.T) {
	h := newHarness(t)

	stranded := h.execution(entity.ExecutionRunning)

	h.executions.EXPECT().
		ExpiredLeases(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entity.Execution{stranded}, nil)

	h.states.EXPECT().ListByTeamID(gomock.Any(), gomock.Any()).Return(h.states34(), nil).AnyTimes()

	var landed entity.ExecutionState

	h.executions.EXPECT().
		Move(gomock.Any(), stranded.ID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, id string, request repository.ExecutionMove,
		) (entity.Execution, error) {
			landed = request.To
			moved := h.execution(request.To)
			moved.ID = id

			return moved, nil
		})

	if err := h.service.SweepLeases(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if landed != entity.ExecutionInterrupted {
		t.Fatalf("a runner that went silent left its run %q, want interrupted", landed)
	}

	if !(entity.Execution{State: landed}).Restartable() {
		t.Fatal("an interrupted run is not offered a restart, so the work would be stranded")
	}
}

func TestASyncNamesEveryRunTheMachineIsStillHolding(t *testing.T) {
	h := newHarness(t)

	h.executions.EXPECT().
		ListLiveByRunner(gomock.Any(), h.runner.ID).
		Return([]entity.Execution{h.execution(entity.ExecutionRunning)}, nil)

	held, err := h.service.Leased(context.Background(), h.runner.ID)
	if err != nil {
		t.Fatalf("list what the machine holds: %v", err)
	}

	if len(held) != 1 || held[0] != "exec-01ABC" {
		t.Fatalf("the machine is told it holds %v, want [exec-01ABC]", held)
	}
}

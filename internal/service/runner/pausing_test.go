package runner_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func (h *harness) onRecord(runner entity.Runner) {
	h.runners.EXPECT().GetByID(gomock.Any(), runner.ID).Return(runner, nil)
}

func (h *harness) standby() *entity.Runner {
	settled := &entity.Runner{}

	h.runners.EXPECT().
		SetPaused(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, runnerID uuid.UUID, pausedAt *time.Time,
		) (entity.Runner, error) {
			settled.ID = runnerID
			settled.WorkspaceID = h.workspaceID
			settled.PausedAt = pausedAt

			return *settled, nil
		})

	return settled
}

func (h *harness) told() *[]entity.ChannelMessage {
	sent := &[]entity.ChannelMessage{}

	h.channels.EXPECT().
		Append(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, message entity.ChannelMessage,
		) (string, error) {
			*sent = append(*sent, message)

			return "1-0", nil
		}).
		AnyTimes()

	return sent
}

func TestPausingAMachineWritesItDownAndTellsTheMachine(t *testing.T) {
	h := newHarness(t)

	machine := h.enrolled(newDevice(t), "nrr_secret")
	h.onRecord(machine)

	settled := h.standby()

	var sent []entity.ChannelMessage

	h.channels.EXPECT().
		Append(gomock.Any(), machine.ID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, message entity.ChannelMessage,
		) (string, error) {
			sent = append(sent, message)

			return "1-0", nil
		})

	paused, err := h.service.Pause(context.Background(), h.workspaceID, machine.ID)
	if err != nil {
		t.Fatalf("pause: %v", err)
	}

	if !paused.Paused() || settled.PausedAt == nil {
		t.Fatal(
			"pausing did not reach the database. A machine somebody paused would take work " +
				"again the moment its daemon restarted, against what they asked for",
		)
	}

	if len(sent) != 1 || sent[0].Type != entity.ChannelRunnerPause {
		t.Fatalf(
			"the machine was told %+v; it has to hear about a pause too, or norn stops "+
				"offering while norn runner status still says it is taking work",
			sent,
		)
	}

	if sent[0].ExecutionID != "" {
		t.Fatalf(
			"a pause named execution %q; it is about the machine, not about one run",
			sent[0].ExecutionID,
		)
	}
}

func TestResumingAMachineAlreadyTakingWorkChangesNothing(t *testing.T) {
	h := newHarness(t)

	machine := h.enrolled(newDevice(t), "nrr_secret")
	h.onRecord(machine)

	sent := h.told()

	if _, err := h.service.Resume(context.Background(), h.workspaceID, machine.ID); err != nil {
		t.Fatalf("resume: %v", err)
	}

	if len(*sent) != 0 {
		t.Fatalf(
			"resuming a machine that was never paused sent it %+v. Repeating the call would "+
				"write a fresh audit entry and wake the machine up for nothing every time",
			*sent,
		)
	}
}

func TestAMachineInAnotherWorkspaceCannotBePaused(t *testing.T) {
	h := newHarness(t)

	machine := h.enrolled(newDevice(t), "nrr_secret")
	machine.WorkspaceID = uuid.New()

	h.onRecord(machine)

	if _, err := h.service.Pause(context.Background(), h.workspaceID, machine.ID); err == nil {
		t.Fatal(
			"a machine belonging to another workspace was paused from this one. Runner ids are " +
				"the only thing between the two, and one workspace stopping another's builds is " +
				"not a mistake anybody would find quickly",
		)
	}
}

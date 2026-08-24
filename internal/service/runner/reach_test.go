package runner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func (h *harness) asMember(accountID uuid.UUID) {
	h.role = entity.MembershipRoleMember
	h.reader = accountID
}

func (h *harness) registered(agents ...entity.Agent) {
	h.agents.EXPECT().
		ListByWorkspaceID(gomock.Any(), h.workspaceID).
		Return(agents, nil).
		AnyTimes()

	for _, agent := range agents {
		h.agents.EXPECT().
			GetByID(gomock.Any(), h.workspaceID, agent.ID).
			Return(agent, nil).
			AnyTimes()
	}
}

func (h *harness) ownedBy(accountID uuid.UUID) entity.Agent {
	return entity.Agent{
		ID:             uuid.New(),
		WorkspaceID:    h.workspaceID,
		AccountID:      uuid.New(),
		OwnerAccountID: accountID,
		Name:           "opsy",
		Status:         entity.AgentStatusActive,
	}
}

func (h *harness) machineOf(t *testing.T, agent entity.Agent) entity.Runner {
	t.Helper()

	machine := h.enrolled(newDevice(t), "nrr_secret")
	machine.AgentID = agent.ID

	return machine
}

func (h *harness) onRecords(machines ...entity.Runner) {
	h.runners.EXPECT().ListByWorkspaceID(gomock.Any(), h.workspaceID).Return(machines, nil)

	for _, machine := range machines {
		h.channels.EXPECT().
			Presence(gomock.Any(), machine.ID).
			Return(entity.RunnerPresence{}, nil).
			AnyTimes()
	}
}

func TestAMemberSeesOnlyTheMachinesOfAgentsTheyOwn(t *testing.T) {
	h := newHarness(t)

	mine := h.ownedBy(uuid.New())
	theirs := h.ownedBy(uuid.New())

	h.asMember(mine.OwnerAccountID)
	h.registered(mine, theirs)

	ours := h.machineOf(t, mine)
	h.onRecords(ours, h.machineOf(t, theirs))

	states, err := h.service.List(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(states) != 1 || states[0].Runner.ID != ours.ID {
		t.Fatalf(
			"a member was shown %d machines. A runner is somebody's own computer, and its "+
				"hostname and folders are not for the rest of the workspace to read",
			len(states),
		)
	}
}

func TestAnAdministratorSeesEveryMachineInTheWorkspace(t *testing.T) {
	h := newHarness(t)

	first := h.ownedBy(uuid.New())
	second := h.ownedBy(uuid.New())

	h.registered(first, second)
	h.onRecords(h.machineOf(t, first), h.machineOf(t, second))

	states, err := h.service.List(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(states) != 2 {
		t.Fatalf(
			"an administrator was shown %d of 2 machines. Revoking a machine somebody left "+
				"running is an administrator's job, and they cannot revoke what they cannot see",
			len(states),
		)
	}
}

func TestAMemberWhoOwnsNoAgentSeesNoMachines(t *testing.T) {
	h := newHarness(t)

	theirs := h.ownedBy(uuid.New())

	h.asMember(uuid.New())
	h.registered(theirs)
	h.onRecords(h.machineOf(t, theirs))

	states, err := h.service.List(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(states) != 0 {
		t.Fatalf("a member who owns no agent was shown %d machines", len(states))
	}
}

func TestTheOwnerOfAnAgentMayPauseItsMachine(t *testing.T) {
	h := newHarness(t)

	mine := h.ownedBy(uuid.New())

	h.asMember(mine.OwnerAccountID)
	h.registered(mine)

	machine := h.machineOf(t, mine)
	h.onRecord(machine)
	h.standby()
	h.told()

	paused, err := h.service.Pause(context.Background(), h.workspaceID, machine.ID)
	if err != nil {
		t.Fatalf("pause: %v", err)
	}

	if !paused.Paused() {
		t.Fatal("the owner of an agent could not stop their own computer taking work")
	}
}

func TestAMemberCannotPauseSomebodyElsesMachine(t *testing.T) {
	h := newHarness(t)

	theirs := h.ownedBy(uuid.New())

	h.asMember(uuid.New())
	h.registered(theirs)

	machine := h.machineOf(t, theirs)
	h.onRecord(machine)

	_, err := h.service.Pause(context.Background(), h.workspaceID, machine.ID)
	if !errors.Is(err, entity.ErrRunnerNotFound) {
		t.Fatalf(
			"pausing a colleague's machine answered %v. It has to read as not found rather "+
				"than as refused, or the answer confirms whose machine exists",
			err,
		)
	}
}

func TestAMemberCannotRevokeSomebodyElsesMachine(t *testing.T) {
	h := newHarness(t)

	theirs := h.ownedBy(uuid.New())

	h.asMember(uuid.New())
	h.registered(theirs)

	machine := h.machineOf(t, theirs)
	h.onRecord(machine)

	if err := h.service.Revoke(
		context.Background(), h.workspaceID, machine.ID,
	); !errors.Is(err, entity.ErrRunnerNotFound) {
		t.Fatalf("revoking a colleague's machine answered %v", err)
	}
}

func TestTheOwnerOfAnAgentMayRevokeItsMachine(t *testing.T) {
	h := newHarness(t)

	mine := h.ownedBy(uuid.New())

	h.asMember(mine.OwnerAccountID)
	h.registered(mine)

	machine := h.machineOf(t, mine)
	h.onRecord(machine)

	h.runners.EXPECT().
		Revoke(gomock.Any(), h.workspaceID, machine.ID, gomock.Any()).
		Return(nil)

	if err := h.service.Revoke(context.Background(), h.workspaceID, machine.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
}

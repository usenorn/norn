package codebase_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func (h *harness) onAgentRecord() {
	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, h.agent.ID).
		Return(h.agent, nil).
		AnyTimes()
}

func TestTheOwnerOfAnAgentMayDisconnectOneOfItsFolders(t *testing.T) {
	h := newHarness(t)

	live := h.live(entity.CodebaseStateActive)
	gone := live
	gone.State = entity.CodebaseStateDisconnected

	h.onAgentRecord()
	h.codebases.EXPECT().GetByID(gomock.Any(), live.ID).Return(live, nil)
	h.runners.EXPECT().GetByID(gomock.Any(), h.runner.ID).Return(h.runner, nil)
	h.codebases.EXPECT().Disconnect(gomock.Any(), live.ID, gomock.Any()).Return(gone, nil)

	codebase, err := h.service.DisconnectAgentCodebase(
		h.asPerson(), h.workspaceID, h.agent.ID, live.ID,
	)
	if err != nil {
		t.Fatalf("the owner disconnecting a folder: %v", err)
	}

	if codebase.State != entity.CodebaseStateDisconnected {
		t.Fatalf(
			"disconnect answered %q. It is a state rather than a deletion, so a run that "+
				"already named this folder keeps resolving",
			codebase.State,
		)
	}
}

func TestAMemberCannotDisconnectAFolderOnSomebodyElsesMachine(t *testing.T) {
	h := newHarness(t)

	h.caller = uuid.New()
	h.onAgentRecord()

	_, err := h.service.DisconnectAgentCodebase(
		h.asPerson(), h.workspaceID, h.agent.ID, uuid.New(),
	)
	if !errors.Is(err, entity.ErrAgentNotFound) {
		t.Fatalf(
			"disconnecting a colleague's folder answered %v, want %v — the answer must not "+
				"confirm that the agent exists",
			err, entity.ErrAgentNotFound,
		)
	}
}

func TestAFolderHeldByAnotherAgentIsNotReachableThroughThisOne(t *testing.T) {
	h := newHarness(t)

	elsewhere := h.live(entity.CodebaseStateActive)
	elsewhere.AgentID = uuid.New()

	h.onAgentRecord()
	h.codebases.EXPECT().GetByID(gomock.Any(), elsewhere.ID).Return(elsewhere, nil)

	_, err := h.service.DisconnectAgentCodebase(
		h.asPerson(), h.workspaceID, h.agent.ID, elsewhere.ID,
	)
	if !errors.Is(err, entity.ErrCodebaseNotFound) {
		t.Fatalf(
			"a folder belonging to another agent answered %v. The codebase id is the only "+
				"thing between the two, and one of them is on a different person's computer",
			err,
		)
	}
}

func TestAFolderAlreadyDisconnectedIsNotDisconnectedTwice(t *testing.T) {
	h := newHarness(t)

	gone := h.live(entity.CodebaseStateDisconnected)

	h.onAgentRecord()
	h.codebases.EXPECT().GetByID(gomock.Any(), gone.ID).Return(gone, nil)

	_, err := h.service.DisconnectAgentCodebase(
		h.asPerson(), h.workspaceID, h.agent.ID, gone.ID,
	)
	if !errors.Is(err, entity.ErrCodebaseDisconnected) {
		t.Fatalf("disconnecting a folder that already was answered %v", err)
	}
}

func TestAnAdministratorMayDisconnectAFolderOnAnAgentTheyDoNotOwn(t *testing.T) {
	h := newHarness(t)

	h.caller = uuid.New()
	h.role = entity.MembershipRoleAdmin

	live := h.live(entity.CodebaseStateActive)
	gone := live
	gone.State = entity.CodebaseStateDisconnected

	h.onAgentRecord()
	h.codebases.EXPECT().GetByID(gomock.Any(), live.ID).Return(live, nil)
	h.runners.EXPECT().GetByID(gomock.Any(), h.runner.ID).Return(h.runner, nil)
	h.codebases.EXPECT().Disconnect(gomock.Any(), live.ID, gomock.Any()).Return(gone, nil)

	if _, err := h.service.DisconnectAgentCodebase(
		h.asPerson(), h.workspaceID, h.agent.ID, live.ID,
	); err != nil {
		t.Fatalf("an administrator disconnecting a folder: %v", err)
	}
}

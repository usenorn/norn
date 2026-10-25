package memoryevidence_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	memoryrepo "github.com/usenorn/norn/internal/repository/memory"
	notificationeventrepo "github.com/usenorn/norn/internal/repository/notificationevent"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	memoryevidencesvc "github.com/usenorn/norn/internal/service/memoryevidence"
)

type harness struct {
	checks      service.MemoryEvidenceChecks
	memories    *memoryrepo.MockMemory
	workspaceID uuid.UUID
	stored      map[uuid.UUID]entity.Memory
	notified    []entity.NotificationEvent
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)
	h := &harness{
		memories:    memoryrepo.NewMockMemory(ctrl),
		workspaceID: uuid.New(),
		stored:      map[uuid.UUID]entity.Memory{},
	}

	notify := notificationeventrepo.NewMockNotificationEvent(ctrl)
	notify.EXPECT().Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, event entity.NotificationEvent) error {
			h.notified = append(h.notified, event)

			return nil
		}).
		AnyTimes()

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()

	h.memories.EXPECT().Unalerted(gomock.Any(), h.workspaceID).
		DoAndReturn(func(context.Context, uuid.UUID) ([]uuid.UUID, error) {
			var ids []uuid.UUID

			for id, memory := range h.stored {
				if slices.ContainsFunc(memory.Evidence, entity.MemoryEvidence.Unalerted) {
					ids = append(ids, id)
				}
			}

			return ids, nil
		}).
		AnyTimes()
	h.memories.EXPECT().LockByID(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, memoryID uuid.UUID) (entity.Memory, error) {
			return h.stored[memoryID], nil
		}).
		AnyTimes()
	h.memories.EXPECT().Alerted(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, memoryID uuid.UUID, position int, state entity.MemoryEvidenceState) error {
			memory := h.stored[memoryID]
			memory.Evidence = slices.Clone(memory.Evidence)
			memory.Evidence[position].AlertedState = state
			h.stored[memoryID] = memory

			return nil
		}).
		AnyTimes()

	h.checks = memoryevidencesvc.New(h.memories, notify, jobqueuerepo.NewMockJobProducer(ctrl), transactor)

	return h
}

func (h *harness) remember(status entity.MemoryStatus, evidence ...entity.MemoryEvidence) entity.Memory {
	memory := entity.Memory{
		ID:             uuid.New(),
		WorkspaceID:    h.workspaceID,
		Status:         status,
		AuthorID:       uuid.New(),
		ContributorIDs: []uuid.UUID{uuid.New()},
		Evidence:       evidence,
	}
	memory.ContributorIDs = append(memory.ContributorIDs, memory.AuthorID)
	h.stored[memory.ID] = memory

	return memory
}

func cited(state entity.MemoryEvidenceState) entity.MemoryEvidence {
	return entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, State: state}
}

func (h *harness) targets() []uuid.UUID {
	var targets []uuid.UUID
	for _, event := range h.notified {
		targets = append(targets, event.Target)
	}

	return targets
}

func TestAChangedSourceNotifiesEachEditorOnce(t *testing.T) {
	h := newHarness(t)
	memory := h.remember(entity.MemoryStatusAccepted,
		cited(entity.MemoryEvidenceCurrent), cited(entity.MemoryEvidenceChanged), cited(entity.MemoryEvidenceChanged))

	for range 2 {
		if err := h.checks.Check(context.Background(), h.workspaceID); err != nil {
			t.Fatalf("check: %v", err)
		}
	}

	want := []uuid.UUID{memory.AuthorID, memory.ContributorIDs[0]}
	if !slices.Equal(h.targets(), want) {
		t.Fatalf("notified %v, want each editor once: %v", h.targets(), want)
	}

	for _, event := range h.notified {
		if event.Kind != entity.NotificationKindMemoryEvidenceChanged || event.Subject != entity.NotifyMemory(memory.ID) ||
			event.ActorKind != entity.ActorKindSystem {
			t.Fatalf("notified %+v", event)
		}
	}

	if stored := h.stored[memory.ID]; stored.Status != entity.MemoryStatusAccepted {
		t.Fatalf("stale evidence moved the memory to %s", stored.Status)
	}
}

func TestASourceThatGoesFromChangedToUnavailableNotifiesAgain(t *testing.T) {
	h := newHarness(t)
	memory := h.remember(entity.MemoryStatusAccepted, cited(entity.MemoryEvidenceChanged))

	if err := h.checks.Check(context.Background(), h.workspaceID); err != nil {
		t.Fatalf("check: %v", err)
	}

	gone := h.stored[memory.ID]
	gone.Evidence = []entity.MemoryEvidence{{
		Kind: entity.MemoryEvidenceIssue, State: entity.MemoryEvidenceUnavailable, AlertedState: entity.MemoryEvidenceChanged,
	}}
	h.stored[memory.ID] = gone

	if err := h.checks.Check(context.Background(), h.workspaceID); err != nil {
		t.Fatalf("check: %v", err)
	}

	if len(h.notified) != 4 {
		t.Fatalf("notified %d times, want both editors on each of the two changes", len(h.notified))
	}
}

func TestAClosedMemoryIsNotFlagged(t *testing.T) {
	h := newHarness(t)
	h.remember(entity.MemoryStatusWithdrawn, cited(entity.MemoryEvidenceChanged))

	if err := h.checks.Check(context.Background(), h.workspaceID); err != nil {
		t.Fatalf("check: %v", err)
	}

	if len(h.notified) != 0 {
		t.Fatalf("a withdrawn memory notified %v", h.targets())
	}
}

func TestTheSweepChecksEveryWorkspaceWithUnalertedEvidence(t *testing.T) {
	h := newHarness(t)
	h.remember(entity.MemoryStatusProposed, cited(entity.MemoryEvidenceUnavailable))
	h.memories.EXPECT().UnalertedWorkspaces(gomock.Any()).Return([]uuid.UUID{h.workspaceID}, nil)

	if err := h.checks.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if len(h.notified) != 2 {
		t.Fatalf("the sweep notified %d editors, want 2", len(h.notified))
	}
}

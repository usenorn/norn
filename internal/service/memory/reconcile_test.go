package memory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func decided(statement, conditions string) service.MemoryContent {
	return service.MemoryContent{
		Kind:       entity.MemoryKindDecision,
		Statement:  statement,
		Rationale:  "the operations team agreed on it",
		Conditions: conditions,
	}
}

func (h *harness) remember(t *testing.T, memory service.MemoryContent) service.MemoryWrite {
	t.Helper()

	written, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: memory,
		Audience:      entity.WorkspaceAudience(),
	})
	if err != nil {
		t.Fatalf("remember %q: %v", memory.Statement, err)
	}

	return written
}

type offer struct {
	resolution entity.MemoryResolution
	target     uuid.UUID
}

func offers(choices []entity.MemoryChoice) []offer {
	var got []offer

	for _, choice := range choices {
		var target uuid.UUID
		if choice.TargetID != nil {
			target = *choice.TargetID
		}

		got = append(got, offer{choice.Resolution, target})
	}

	return got
}

func relations(related []entity.RelatedMemory) map[uuid.UUID]entity.MemoryRelation {
	got := map[uuid.UUID]entity.MemoryRelation{}

	for _, candidate := range related {
		got[candidate.Memory.ID] = candidate.Relation
	}

	return got
}

func TestEachKindOfRelatedMemoryHoldsTheDraftWithTheRightChoices(t *testing.T) {
	for name, testCase := range map[string]struct {
		existing service.MemoryContent
		draft    service.MemoryContent
		relation entity.MemoryRelation
		status   entity.MemoryStatus
	}{
		"a duplicate": {
			existing: decided("Production uses PostgreSQL", "all deployed environments"),
			draft:    decided("production uses PostgreSQL.", "All deployed environments"),
			relation: entity.MemoryRelationDuplicate,
			status:   entity.MemoryStatusProposed,
		},
		"the same subject under other conditions": {
			existing: decided("Use PostgreSQL for the database", "production"),
			draft:    decided("Use SQLite for the database", "local prototypes"),
			relation: entity.MemoryRelationConditional,
			status:   entity.MemoryStatusProposed,
		},
		"a conflicting accepted decision": {
			existing: decided("Production uses PostgreSQL", "all deployed environments"),
			draft:    decided("Production uses MySQL", "all deployed environments"),
			relation: entity.MemoryRelationConflict,
			status:   entity.MemoryStatusDisputed,
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			existing := h.remember(t, testCase.existing).Memory
			h.remember(t, decided("Releases ship on Tuesdays", ""))

			written := h.remember(t, testCase.draft)

			if written.Memory.Status != testCase.status || h.stored[written.Memory.ID].Status != testCase.status {
				t.Fatalf("the draft was stored %s, want %s", h.stored[written.Memory.ID].Status, testCase.status)
			}

			got := relations(written.Related)
			if len(got) != 1 || got[existing.ID] != testCase.relation {
				t.Fatalf("related = %v, want only %s as %s", got, existing.ID, testCase.relation)
			}

			want := []offer{
				{entity.MemoryResolutionIndependent, uuid.Nil},
				{entity.MemoryResolutionUpdate, existing.ID},
				{entity.MemoryResolutionSupersede, existing.ID},
				{entity.MemoryResolutionNoChange, uuid.Nil},
			}

			if choices := offers(written.Choices); !equalOffers(choices, want) {
				t.Fatalf("choices = %v, want %v", choices, want)
			}

			if existing := h.stored[existing.ID]; existing.Status != entity.MemoryStatusAccepted || existing.Version != 1 {
				t.Fatalf("the existing memory became %s at version %d; nothing may be resolved automatically",
					existing.Status, existing.Version)
			}
		})
	}
}

func equalOffers(a, b []offer) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestAMemoryWithNothingRelatedIsAcceptedWithoutChoices(t *testing.T) {
	h := newHarness(t)
	h.remember(t, decided("Production uses PostgreSQL", ""))

	written := h.remember(t, decided("Releases ship on Tuesdays", ""))

	if written.Memory.Status != entity.MemoryStatusAccepted || len(written.Related) != 0 || written.Choices != nil {
		t.Fatalf("got %s with %d related and choices %v, want accepted alone", written.Memory.Status, len(written.Related), written.Choices)
	}

	query := h.searched[len(h.searched)-1]
	if query.Query.Raw != "Releases ship on Tuesdays" || query.Limit != entity.MemoryRelatedLaneSize ||
		query.PersonalAuthorID != h.reader.Identity.Principal {
		t.Fatalf("searched with %+v", query)
	}
}

func TestOnlyMemoriesTheWriterCanSeeAreOffered(t *testing.T) {
	h := newHarness(t)
	hidden := entity.Memory{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		Kind:        entity.MemoryKindDecision,
		Statement:   "Production uses PostgreSQL",
		Audience:    entity.PersonalAudience(),
		AuthorID:    uuid.New(),
		Status:      entity.MemoryStatusAccepted,
		Version:     1,
	}
	h.stored[hidden.ID] = hidden

	written := h.remember(t, decided("Production uses MySQL", ""))

	if written.Memory.Status != entity.MemoryStatusAccepted || len(written.Related) != 0 {
		t.Fatalf("another person's private memory surfaced: %+v", written.Related)
	}
}

func TestAWriterWhoCannotEditTheExistingMemoryMayOnlyKeepOrDropTheirOwn(t *testing.T) {
	h := newHarness(t)
	existing := h.remember(t, decided("Production uses PostgreSQL", "")).Memory

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleMember)
	written := h.remember(t, decided("Production uses MySQL", ""))

	want := []offer{{entity.MemoryResolutionIndependent, uuid.Nil}, {entity.MemoryResolutionNoChange, uuid.Nil}}

	if choices := offers(written.Choices); !equalOffers(choices, want) {
		t.Fatalf("choices = %v, want %v", choices, want)
	}

	if relations(written.Related)[existing.ID] != entity.MemoryRelationConflict {
		t.Fatalf("related = %v, want the existing decision as a conflict", relations(written.Related))
	}
}

func TestAParaphraseIsFoundOnlyThroughTheEmbedding(t *testing.T) {
	h := newHarness(t)
	existing := h.remember(t, decided("Production uses PostgreSQL", "")).Memory

	paraphrase := decided("Our live database is Postgres", "")

	if written := h.remember(t, paraphrase); len(written.Related) != 0 {
		t.Fatalf("word matching alone related %v", relations(written.Related))
	}

	h.embedding = entity.KnowledgeSemanticUsed
	h.nearest = []uuid.UUID{existing.ID}

	written := h.remember(t, paraphrase)

	if relations(written.Related)[existing.ID] != entity.MemoryRelationConflict || written.Memory.Status != entity.MemoryStatusDisputed {
		t.Fatalf("got %s related %v, want disputed against the existing decision",
			written.Memory.Status, relations(written.Related))
	}
}

func TestAPendingMemoryShowsItsRelatedMemoriesAgain(t *testing.T) {
	h := newHarness(t)
	existing := h.remember(t, decided("Production uses PostgreSQL", "")).Memory
	draft := h.remember(t, decided("Production uses MySQL", "")).Memory

	again, err := h.service.Related(context.Background(), h.workspaceID, draft.ID)
	if err != nil {
		t.Fatalf("related: %v", err)
	}

	if got := relations(again.Related); len(got) != 1 || got[existing.ID] != entity.MemoryRelationConflict {
		t.Fatalf("related = %v, want the existing decision only", got)
	}

	settled, err := h.service.Related(context.Background(), h.workspaceID, existing.ID)
	if err != nil {
		t.Fatalf("related of an accepted memory: %v", err)
	}

	if len(settled.Related) != 0 || settled.Choices != nil {
		t.Fatalf("an accepted memory was offered %v", settled.Choices)
	}
}

func (h *harness) reconcile(
	memory entity.Memory,
	resolution entity.MemoryResolution,
	target *entity.Memory,
) (service.MemoryReconciliation, error) {
	input := service.ReconcileMemoryInput{Resolution: resolution, ExpectedVersion: memory.Version}
	if target != nil {
		input.Target = &service.MemoryTarget{ID: target.ID, ExpectedVersion: target.Version}
	}

	return h.service.Reconcile(context.Background(), h.workspaceID, memory.ID, input)
}

func (h *harness) conflicting(t *testing.T) (entity.Memory, entity.Memory) {
	t.Helper()

	existing := h.remember(t, decided("Production uses PostgreSQL", "all deployed environments")).Memory
	draft := h.remember(t, decided("Production uses MySQL", "all deployed environments")).Memory

	return existing, draft
}

func lastAction(h *harness, memoryID uuid.UUID) entity.MemoryRevisionAction {
	revisions := h.revisionsOf(memoryID)

	return revisions[len(revisions)-1].Action
}

func TestSupersedingFromReconciliationLinksBothMemories(t *testing.T) {
	h := newHarness(t)
	existing, draft := h.conflicting(t)

	reconciled, err := h.reconcile(draft, entity.MemoryResolutionSupersede, &existing)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	replaced, replacement := h.stored[existing.ID], h.stored[draft.ID]

	if replaced.Status != entity.MemoryStatusSuperseded || replaced.SupersededByID == nil || *replaced.SupersededByID != draft.ID {
		t.Fatalf("the existing decision is %s replaced by %v, want superseded by %s", replaced.Status, replaced.SupersededByID, draft.ID)
	}

	if replacement.Status != entity.MemoryStatusAccepted || replacement.SupersedesID == nil || *replacement.SupersedesID != existing.ID {
		t.Fatalf("the draft is %s superseding %v, want accepted superseding %s", replacement.Status, replacement.SupersedesID, existing.ID)
	}

	if reconciled.Memory.ID != draft.ID || reconciled.Target == nil || reconciled.Target.ID != existing.ID {
		t.Fatalf("reconciliation answered with %s and %v", reconciled.Memory.ID, reconciled.Target)
	}

	if lastAction(h, existing.ID) != entity.MemoryRevisionSuperseded || lastAction(h, draft.ID) != entity.MemoryRevisionAccepted {
		t.Fatalf("recorded %s and %s", lastAction(h, existing.ID), lastAction(h, draft.ID))
	}

	lineage, err := h.service.Lineage(context.Background(), h.workspaceID, draft.ID)
	if err != nil {
		t.Fatalf("lineage: %v", err)
	}

	if len(lineage) != 2 || lineage[0].ID != existing.ID || lineage[1].ID != draft.ID {
		t.Fatalf("lineage = %v, want the existing decision then its replacement", lineage)
	}
}

func TestKeepingADraftIndependentAcceptsItAndLeavesTheOtherAlone(t *testing.T) {
	h := newHarness(t)
	existing, draft := h.conflicting(t)

	reconciled, err := h.reconcile(draft, entity.MemoryResolutionIndependent, nil)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if reconciled.Memory.Status != entity.MemoryStatusAccepted || reconciled.Memory.Version != 2 || reconciled.Target != nil {
		t.Fatalf("the draft became %s at version %d", reconciled.Memory.Status, reconciled.Memory.Version)
	}

	if lastAction(h, draft.ID) != entity.MemoryRevisionAccepted {
		t.Fatalf("recorded %s", lastAction(h, draft.ID))
	}

	if h.stored[existing.ID].Version != 1 || h.stored[existing.ID].Status != entity.MemoryStatusAccepted {
		t.Fatalf("the existing decision changed to %+v", h.stored[existing.ID])
	}
}

func TestMakingNoChangeWithdrawsOnlyTheDraft(t *testing.T) {
	h := newHarness(t)
	existing, draft := h.conflicting(t)

	reconciled, err := h.reconcile(draft, entity.MemoryResolutionNoChange, nil)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if reconciled.Memory.Status != entity.MemoryStatusWithdrawn || lastAction(h, draft.ID) != entity.MemoryRevisionWithdrawn {
		t.Fatalf("the draft became %s", reconciled.Memory.Status)
	}

	if h.stored[existing.ID].Version != 1 {
		t.Fatalf("the existing decision changed to version %d", h.stored[existing.ID].Version)
	}
}

func TestUpdatingCorrectsTheExistingMemoryWithTheDraftAndRetiresTheDraft(t *testing.T) {
	h := newHarness(t)
	existing, draft := h.conflicting(t)

	reconciled, err := h.reconcile(draft, entity.MemoryResolutionUpdate, &existing)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	corrected := h.stored[existing.ID]
	if corrected.Statement != "Production uses MySQL" || corrected.Version != 2 || corrected.Status != entity.MemoryStatusAccepted {
		t.Fatalf("the existing memory is %q at version %d, %s", corrected.Statement, corrected.Version, corrected.Status)
	}

	if lastAction(h, existing.ID) != entity.MemoryRevisionCorrected || reconciled.Target == nil || reconciled.Target.Version != 2 {
		t.Fatalf("recorded %s, answered target %v", lastAction(h, existing.ID), reconciled.Target)
	}

	if h.stored[draft.ID].Status != entity.MemoryStatusWithdrawn {
		t.Fatalf("the draft stayed %s", h.stored[draft.ID].Status)
	}
}

func TestReconciliationIsRefusedWhenItCannotApply(t *testing.T) {
	h := newHarness(t)
	existing, draft := h.conflicting(t)
	stale := draft
	stale.Version = 7
	staleTarget := existing
	staleTarget.Version = 3

	var refusal entity.ValidationError

	for name, testCase := range map[string]struct {
		run   func() error
		check func(error) bool
	}{
		"an accepted memory": {
			run:   func() error { _, err := h.reconcile(existing, entity.MemoryResolutionIndependent, nil); return err },
			check: func(err error) bool { return errors.Is(err, entity.ErrMemoryNotPending) },
		},
		"a stale draft": {
			run:   func() error { _, err := h.reconcile(stale, entity.MemoryResolutionIndependent, nil); return err },
			check: func(err error) bool { return errors.Is(err, entity.ErrMemoryStale) },
		},
		"a stale target": {
			run:   func() error { _, err := h.reconcile(draft, entity.MemoryResolutionSupersede, &staleTarget); return err },
			check: func(err error) bool { return errors.Is(err, entity.ErrMemoryStale) },
		},
		"supersede without a target": {
			run:   func() error { _, err := h.reconcile(draft, entity.MemoryResolutionSupersede, nil); return err },
			check: func(err error) bool { return errors.As(err, &refusal) },
		},
		"a target the draft itself": {
			run:   func() error { _, err := h.reconcile(draft, entity.MemoryResolutionUpdate, &draft); return err },
			check: func(err error) bool { return errors.As(err, &refusal) },
		},
		"a target for keeping it independent": {
			run:   func() error { _, err := h.reconcile(draft, entity.MemoryResolutionIndependent, &existing); return err },
			check: func(err error) bool { return errors.As(err, &refusal) },
		},
		"an unknown resolution": {
			run:   func() error { _, err := h.reconcile(draft, "merge", nil); return err },
			check: func(err error) bool { return errors.As(err, &refusal) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := testCase.run(); !testCase.check(err) {
				t.Fatalf("got %v", err)
			}
		})
	}

	if h.stored[draft.ID].Status != entity.MemoryStatusDisputed || h.stored[existing.ID].Version != 1 {
		t.Fatal("a refused reconciliation changed a memory")
	}
}

func TestOnlySomeoneWhoMayEditTheExistingMemoryMaySupersedeIt(t *testing.T) {
	h := newHarness(t)
	existing := h.remember(t, decided("Production uses PostgreSQL", "")).Memory

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleMember)
	draft := h.remember(t, decided("Production uses MySQL", "")).Memory

	if _, err := h.reconcile(draft, entity.MemoryResolutionSupersede, &existing); !errors.Is(err, entity.ErrKnowledgeNotEditable) {
		t.Fatalf("got %v, want not editable", err)
	}

	if h.stored[existing.ID].Status != entity.MemoryStatusAccepted {
		t.Fatalf("the existing decision became %s", h.stored[existing.ID].Status)
	}
}

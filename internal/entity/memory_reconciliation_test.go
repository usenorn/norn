package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func decision(statement, conditions string) entity.Memory {
	return entity.Memory{
		ID:         uuid.New(),
		Kind:       entity.MemoryKindDecision,
		Statement:  statement,
		Conditions: conditions,
		Status:     entity.MemoryStatusAccepted,
		Viewer:     entity.MemoryViewer{Edits: true},
	}
}

func TestADraftIsRelatedByDuplicationConditionOrConflict(t *testing.T) {
	projectID, otherProjectID := uuid.New(), uuid.New()

	for name, testCase := range map[string]struct {
		draft    entity.Memory
		existing entity.Memory
		semantic bool
		want     entity.MemoryRelation
		related  bool
	}{
		"the same statement reworded only in case and punctuation": {
			draft:    decision("Production uses PostgreSQL.", "all deployed environments"),
			existing: decision("production uses postgresql", "All deployed environments"),
			want:     entity.MemoryRelationDuplicate,
			related:  true,
		},
		"a near restatement of a lesson": {
			draft:    entity.Memory{Kind: entity.MemoryKindLesson, Statement: "Flaky upload tests come from the shared bucket"},
			existing: entity.Memory{Kind: entity.MemoryKindLesson, Statement: "Upload tests are flaky because of the shared bucket", Status: entity.MemoryStatusAccepted},
			want:     entity.MemoryRelationDuplicate,
			related:  true,
		},
		"the same subject under other conditions": {
			draft:    decision("Use SQLite for the database", "local prototypes"),
			existing: decision("Use PostgreSQL for the database", "production"),
			want:     entity.MemoryRelationConditional,
			related:  true,
		},
		"contradicting an accepted decision under the same conditions": {
			draft:    decision("Production uses MySQL", "all deployed environments"),
			existing: decision("Production uses PostgreSQL", "all deployed environments"),
			want:     entity.MemoryRelationConflict,
			related:  true,
		},
		"contradicting a decision that is only proposed": {
			draft: decision("Production uses MySQL", ""),
			existing: func() entity.Memory {
				m := decision("Production uses PostgreSQL", "")
				m.Status = entity.MemoryStatusProposed
				return m
			}(),
			want:    entity.MemoryRelationDuplicate,
			related: true,
		},
		"an unrelated statement": {
			draft:    decision("Releases ship on Tuesdays", ""),
			existing: decision("Production uses PostgreSQL", ""),
		},
		"a paraphrase the embedding found": {
			draft:    decision("Our live database is Postgres", ""),
			existing: decision("Production uses PostgreSQL", ""),
			semantic: true,
			want:     entity.MemoryRelationConflict,
			related:  true,
		},
		"another project's decision": {
			draft: func() entity.Memory {
				m := decision("Production uses MySQL", "")
				m.Applicability.ProjectID = &projectID
				return m
			}(),
			existing: func() entity.Memory {
				m := decision("Production uses PostgreSQL", "")
				m.Applicability.ProjectID = &otherProjectID
				return m
			}(),
		},
		"a workspace-wide decision seen from a project": {
			draft: func() entity.Memory {
				m := decision("Production uses MySQL", "")
				m.Applicability.ProjectID = &projectID
				return m
			}(),
			existing: decision("Production uses PostgreSQL", ""),
			want:     entity.MemoryRelationConflict,
			related:  true,
		},
		"another component": {
			draft: func() entity.Memory {
				m := decision("Production uses MySQL", "")
				m.Applicability.Component = "billing"
				return m
			}(),
			existing: func() entity.Memory {
				m := decision("Production uses PostgreSQL", "")
				m.Applicability.Component = "search"
				return m
			}(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, related := testCase.draft.RelationTo(testCase.existing, testCase.semantic)

			if related != testCase.related || got != testCase.want {
				t.Fatalf("relation = %q related %v, want %q related %v", got, related, testCase.want, testCase.related)
			}
		})
	}
}

func TestADraftWaitsWhileAnythingRelatesAndIsDisputedOnlyByAConflict(t *testing.T) {
	var draft entity.Memory

	for name, testCase := range map[string]struct {
		relations []entity.MemoryRelation
		want      entity.MemoryStatus
	}{
		"nothing related":           {want: entity.MemoryStatusAccepted},
		"a duplicate":               {relations: []entity.MemoryRelation{entity.MemoryRelationDuplicate}, want: entity.MemoryStatusProposed},
		"a compatible condition":    {relations: []entity.MemoryRelation{entity.MemoryRelationConditional}, want: entity.MemoryStatusProposed},
		"a conflict among the rest": {relations: []entity.MemoryRelation{entity.MemoryRelationConditional, entity.MemoryRelationConflict}, want: entity.MemoryStatusDisputed},
	} {
		t.Run(name, func(t *testing.T) {
			var related []entity.RelatedMemory
			for _, relation := range testCase.relations {
				related = append(related, entity.RelatedMemory{Relation: relation})
			}

			if got := draft.StatusAmong(related, true); got != testCase.want {
				t.Fatalf("status = %s, want %s", got, testCase.want)
			}
		})
	}
}

func TestADraftNobodyMaySettleWaitsForReviewUnlessItConflicts(t *testing.T) {
	var draft entity.Memory

	if got := draft.StatusAmong(nil, false); got != entity.MemoryStatusProposed {
		t.Fatalf("an unrelated draft awaiting review is %s, want proposed", got)
	}

	conflict := []entity.RelatedMemory{{Relation: entity.MemoryRelationConflict}}
	if got := draft.StatusAmong(conflict, false); got != entity.MemoryStatusDisputed {
		t.Fatalf("a conflicting draft awaiting review is %s, want disputed", got)
	}
}

func TestChoicesOfferOnlyWhatTheWriterMayDoToEachCandidate(t *testing.T) {
	accepted := decision("Production uses PostgreSQL", "")
	proposed := decision("Production uses Postgres 17", "")
	proposed.Status = entity.MemoryStatusProposed
	readOnly := decision("Production runs one primary", "")
	readOnly.Viewer.Edits = false

	choices := entity.MemoryChoices([]entity.RelatedMemory{
		{Memory: accepted, Relation: entity.MemoryRelationConflict},
		{Memory: proposed, Relation: entity.MemoryRelationDuplicate},
		{Memory: readOnly, Relation: entity.MemoryRelationDuplicate},
	})

	type offer struct {
		resolution entity.MemoryResolution
		target     uuid.UUID
	}

	var got []offer
	for _, choice := range choices {
		var target uuid.UUID
		if choice.TargetID != nil {
			target = *choice.TargetID
		}

		got = append(got, offer{choice.Resolution, target})
	}

	want := []offer{
		{entity.MemoryResolutionIndependent, uuid.Nil},
		{entity.MemoryResolutionUpdate, accepted.ID},
		{entity.MemoryResolutionSupersede, accepted.ID},
		{entity.MemoryResolutionUpdate, proposed.ID},
		{entity.MemoryResolutionNoChange, uuid.Nil},
	}

	if len(got) != len(want) {
		t.Fatalf("choices = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("choice %d = %v, want %v", i, got[i], want[i])
		}
	}

	if entity.MemoryChoices(nil) != nil {
		t.Fatal("a memory with nothing related was offered choices")
	}
}

package entity_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func memoryCandidate(id uuid.UUID, exact bool) entity.KnowledgeCandidate {
	return entity.KnowledgeCandidate{Kind: entity.KnowledgeSubjectMemory, ID: id, Exact: exact}
}

func TestAnExactMatchOutranksAnythingFusedFromBothLanes(t *testing.T) {
	both, exact, semantic := uuid.New(), uuid.New(), uuid.New()

	fused := entity.FuseKnowledge(10,
		entity.KnowledgeLane{Match: entity.KnowledgeMatchLexical, Candidates: []entity.KnowledgeCandidate{
			memoryCandidate(both, false), memoryCandidate(exact, true),
		}},
		entity.KnowledgeLane{Match: entity.KnowledgeMatchSemantic, Candidates: []entity.KnowledgeCandidate{
			memoryCandidate(both, false), memoryCandidate(semantic, false),
		}},
	)

	var order []uuid.UUID
	for _, hit := range fused {
		order = append(order, hit.ID)
	}

	if !slices.Equal(order, []uuid.UUID{exact, both, semantic}) {
		t.Fatalf("order = %v, want the exact match, then the one both lanes found, then the rest", order)
	}

	if !slices.Equal(fused[0].Matches, []entity.KnowledgeMatch{entity.KnowledgeMatchExact, entity.KnowledgeMatchLexical}) {
		t.Errorf("exact hit matched by %v", fused[0].Matches)
	}

	if !slices.Equal(fused[1].Matches, []entity.KnowledgeMatch{entity.KnowledgeMatchLexical, entity.KnowledgeMatchSemantic}) {
		t.Errorf("hit found twice matched by %v, want both lanes named", fused[1].Matches)
	}
}

func TestFusionIsDeterministicAndHonoursTheLimit(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}

	lexical := entity.KnowledgeLane{Match: entity.KnowledgeMatchLexical}
	semantic := entity.KnowledgeLane{Match: entity.KnowledgeMatchSemantic}

	for index, id := range ids {
		lexical.Candidates = append(lexical.Candidates, memoryCandidate(id, false))
		semantic.Candidates = append(semantic.Candidates, memoryCandidate(ids[len(ids)-1-index], false))
	}

	first := entity.FuseKnowledge(3, lexical, semantic)
	second := entity.FuseKnowledge(3, semantic, lexical)

	if len(first) != 3 {
		t.Fatalf("got %d hits, want the limit of 3", len(first))
	}

	for index := range first {
		if first[index].ID != second[index].ID {
			t.Fatalf("lane order changed the result: %v vs %v", first[index].ID, second[index].ID)
		}
	}
}

func TestAKnowledgeCursorRoundTripsAndRefusesGarbage(t *testing.T) {
	cursor := entity.KnowledgeCursor{MemoryID: uuid.New()}

	decoded, err := entity.DecodeKnowledgeCursor(cursor.Encode())
	if err != nil || decoded != cursor {
		t.Fatalf("decoded %v, %v; want %v", decoded, err, cursor)
	}

	for _, raw := range []string{"%%%", "c2hvcnQ"} {
		if _, err := entity.DecodeKnowledgeCursor(raw); err != entity.ErrKnowledgeCursorInvalid {
			t.Errorf("decode %q: err = %v, want ErrKnowledgeCursorInvalid", raw, err)
		}
	}
}

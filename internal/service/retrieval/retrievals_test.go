package retrieval_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	aimodelrepo "github.com/usenorn/norn/internal/repository/aimodel"
	aiproviderrepo "github.com/usenorn/norn/internal/repository/aiprovider"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	knowledgerepo "github.com/usenorn/norn/internal/repository/knowledge"
	memoryrepo "github.com/usenorn/norn/internal/repository/memory"
	memoryrevisionrepo "github.com/usenorn/norn/internal/repository/memoryrevision"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	knowledgeauthorizersvc "github.com/usenorn/norn/internal/service/knowledgeauthorizer"
	knowledgeembeddingsvc "github.com/usenorn/norn/internal/service/knowledgeembedding"
	retrievalsvc "github.com/usenorn/norn/internal/service/retrieval"
)

type harness struct {
	knowledge *knowledgerepo.MockKnowledge
	memories  *memoryrepo.MockMemory
	revisions *memoryrevisionrepo.MockMemoryRevision
	providers *aiproviderrepo.MockAIProvider
	models    *aimodelrepo.MockAIModel
	service   service.Retrievals

	workspaceID uuid.UUID
	accountID   uuid.UUID
	reader      entity.KnowledgeReader
	stored      map[uuid.UUID]entity.Memory
	visible     []uuid.UUID
	lexical     []entity.KnowledgeCandidate
	asked       []repository.KnowledgeQuery
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		knowledge:   knowledgerepo.NewMockKnowledge(ctrl),
		memories:    memoryrepo.NewMockMemory(ctrl),
		revisions:   memoryrevisionrepo.NewMockMemoryRevision(ctrl),
		providers:   aiproviderrepo.NewMockAIProvider(ctrl),
		models:      aimodelrepo.NewMockAIModel(ctrl),
		workspaceID: uuid.New(),
		accountID:   uuid.New(),
		stored:      map[uuid.UUID]entity.Memory{},
	}

	h.reader = entity.KnowledgeReader{
		WorkspaceID: h.workspaceID,
		Identity:    entity.KnowledgeIdentity{Principal: h.accountID, Personal: true},
	}

	readers := knowledgeauthorizersvc.NewMockKnowledgeAuthorizer(ctrl)
	readers.EXPECT().Reader(gomock.Any(), h.workspaceID).
		DoAndReturn(func(context.Context, uuid.UUID) (entity.KnowledgeDecision, error) {
			return entity.KnowledgeDecision{Reader: h.reader}, nil
		}).
		AnyTimes()

	h.knowledge.EXPECT().LexicalMemories(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, query repository.KnowledgeQuery) ([]entity.KnowledgeCandidate, error) {
			h.asked = append(h.asked, query)

			return h.lexical, nil
		}).
		AnyTimes()
	h.knowledge.EXPECT().LexicalPassages(gomock.Any(), h.workspaceID, gomock.Any()).Return(nil, nil).AnyTimes()
	h.knowledge.EXPECT().SemanticPassages(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	h.memories.EXPECT().ListByIDs(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]entity.Memory, error) {
			var found []entity.Memory

			for _, id := range ids {
				if memory, ok := h.stored[id]; ok {
					found = append(found, memory)
				}
			}

			return found, nil
		}).
		AnyTimes()

	issues := issuerepo.NewMockIssue(ctrl)
	issues.EXPECT().ListVisibleByIDs(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, issueIDs []uuid.UUID) ([]entity.Issue, error) {
			var visible []entity.Issue

			for _, issueID := range issueIDs {
				if slices.Contains(h.visible, issueID) {
					visible = append(visible, entity.Issue{ID: issueID})
				}
			}

			return visible, nil
		}).
		AnyTimes()

	authorizer := authorizersvc.NewMockAuthorizer(ctrl)
	authorizer.EXPECT().Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{Scope: entity.TeamScope{WorkspaceID: h.workspaceID}}, nil).
		AnyTimes()

	cfg := config.Knowledge{QueryTimeout: time.Second, SemanticMinSimilarity: 0.3}
	h.service = retrievalsvc.New(
		h.knowledge, h.memories, h.revisions, issues, knowledgeembeddingsvc.New(h.providers, h.models, cfg), readers,
		authorizer, cfg,
	)

	return h
}

func (h *harness) memory(statement string, audience entity.KnowledgeAudience, status entity.MemoryStatus) uuid.UUID {
	memory := entity.Memory{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		Kind:        entity.MemoryKindConstraint,
		Statement:   statement,
		Audience:    audience,
		Status:      status,
		AuthorID:    h.accountID,
		Evidence: []entity.MemoryEvidence{{
			Kind: entity.MemoryEvidenceExcerpt, Excerpt: "from the incident", State: entity.MemoryEvidenceCurrent,
		}},
	}

	h.stored[memory.ID] = memory

	return memory.ID
}

func summarized(evidence []entity.MemoryEvidence) bool {
	for _, cited := range evidence {
		if cited != cited.Summary() || !cited.State.Valid() {
			return false
		}
	}

	return true
}

func (h *harness) noProvider() {
	h.providers.EXPECT().Get(gomock.Any(), h.workspaceID).
		Return(entity.AIProviderConnection{}, entity.ErrAIProviderNotConfigured).AnyTimes()
}

func (h *harness) embeddingProvider(embedErr error) {
	h.providers.EXPECT().Get(gomock.Any(), h.workspaceID).
		Return(entity.AIProviderConnection{WorkspaceID: h.workspaceID, EmbeddingModel: "text-embedding-3-small"}, nil).
		AnyTimes()
	h.providers.EXPECT().APIKey(gomock.Any(), h.workspaceID).Return("sk-live", nil).AnyTimes()
	h.models.EXPECT().Embed(gomock.Any(), gomock.Any(), "sk-live", "text-embedding-3-small", []string{"when do we ship"}).
		DoAndReturn(func(context.Context, entity.AIProviderEndpoint, string, string, []string) ([][]float32, error) {
			if embedErr != nil {
				return nil, embedErr
			}

			return [][]float32{{1, 0}}, nil
		}).
		AnyTimes()
}

func memories(ids ...uuid.UUID) []entity.KnowledgeCandidate {
	candidates := make([]entity.KnowledgeCandidate, len(ids))
	for index, id := range ids {
		candidates[index] = entity.KnowledgeCandidate{Kind: entity.KnowledgeSubjectMemory, ID: id}
	}

	return candidates
}

func hitIDs(retrieval entity.KnowledgeRetrieval) []uuid.UUID {
	ids := make([]uuid.UUID, len(retrieval.Hits))
	for index, hit := range retrieval.Hits {
		ids[index] = hit.ID
	}

	return ids
}

func TestARetrievalNeedsSomethingToLookFor(t *testing.T) {
	h := newHarness(t)

	for name, input := range map[string]service.RetrieveInput{
		"no question":   {Query: "   "},
		"unknown kind":  {Query: "deploys", Kinds: []entity.KnowledgeSubjectKind{"issue"}},
		"unknown class": {Query: "deploys", MemoryKinds: []entity.MemoryKind{"rumour"}},
	} {
		var validation entity.ValidationError

		if _, err := h.service.Retrieve(context.Background(), h.workspaceID, input); !errors.As(err, &validation) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
	}
}

func TestWithoutAnEmbeddingModelRetrievalMatchesWordsAndSaysSo(t *testing.T) {
	h := newHarness(t)
	h.noProvider()

	shipping := h.memory("Ship on Tuesdays", entity.WorkspaceAudience(), entity.MemoryStatusAccepted)
	h.lexical = memories(shipping)

	retrieval, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{Query: "when do we ship"})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if retrieval.Semantic != entity.KnowledgeSemanticUnconfigured || !slices.Equal(hitIDs(retrieval), []uuid.UUID{shipping}) {
		t.Fatalf("semantic %q hits %v, want the word match labelled as word matching only", retrieval.Semantic, hitIDs(retrieval))
	}
}

func TestAnEmbeddingFailureFallsBackToWordMatching(t *testing.T) {
	h := newHarness(t)
	h.embeddingProvider(entity.ErrAIProviderRateLimited)

	shipping := h.memory("Ship on Tuesdays", entity.WorkspaceAudience(), entity.MemoryStatusAccepted)
	h.lexical = memories(shipping)

	retrieval, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{Query: "when do we ship"})
	if err != nil {
		t.Fatalf("Retrieve: %v; a provider hiccup must not take retrieval down", err)
	}

	if retrieval.Semantic != entity.KnowledgeSemanticUnavailable || !slices.Equal(hitIDs(retrieval), []uuid.UUID{shipping}) {
		t.Fatalf("semantic %q hits %v", retrieval.Semantic, hitIDs(retrieval))
	}
}

func TestMeaningAndWordsAreFusedIntoOneRanking(t *testing.T) {
	h := newHarness(t)
	h.embeddingProvider(nil)

	both := h.memory("Ship on Tuesdays", entity.WorkspaceAudience(), entity.MemoryStatusAccepted)
	words := h.memory("Ship notes go in the changelog", entity.WorkspaceAudience(), entity.MemoryStatusAccepted)
	meaning := h.memory("Releases leave every Tuesday", entity.WorkspaceAudience(), entity.MemoryStatusAccepted)
	h.lexical = memories(words, both)

	var nearest repository.KnowledgeVector

	h.knowledge.EXPECT().SemanticMemories(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, _ repository.KnowledgeQuery, vector repository.KnowledgeVector) ([]entity.KnowledgeCandidate, error) {
			nearest = vector

			return memories(both, meaning), nil
		})

	retrieval, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{Query: "when do we ship"})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if retrieval.Semantic != entity.KnowledgeSemanticUsed || hitIDs(retrieval)[0] != both || len(retrieval.Hits) != 3 {
		t.Fatalf("semantic %q hits %v, want the memory both lanes found first", retrieval.Semantic, hitIDs(retrieval))
	}

	if nearest.Model != "text-embedding-3-small" || nearest.MinSimilarity != 0.3 || !slices.Equal(nearest.Vector, []float32{1, 0}) {
		t.Fatalf("searched by meaning with %+v", nearest)
	}

	for _, hit := range retrieval.Hits {
		if hit.Memory == nil || len(hit.Memory.Evidence) != 1 || !summarized(hit.Memory.Evidence) {
			t.Fatalf("hit %v carries %+v; recall answers each source with only its kind and state", hit.ID, hit.Memory)
		}
	}
}

func TestProposalsComeOnlyWhenAskedForAndAreLabelled(t *testing.T) {
	h := newHarness(t)
	h.noProvider()

	proposal := h.memory("Ship on Fridays", entity.WorkspaceAudience(), entity.MemoryStatusProposed)
	h.lexical = memories(proposal)

	if _, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{Query: "ship"}); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	retrieval, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{
		Query: "ship", IncludeProposed: true,
	})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if !slices.Equal(h.asked[0].Statuses, []entity.MemoryStatus{entity.MemoryStatusAccepted}) {
		t.Errorf("by default asked for %v, want accepted guidance only", h.asked[0].Statuses)
	}

	if !slices.Contains(h.asked[1].Statuses, entity.MemoryStatusProposed) {
		t.Errorf("when asked asked for %v, want proposals included", h.asked[1].Statuses)
	}

	if len(retrieval.Hits) != 1 || !retrieval.Hits[0].Proposed {
		t.Fatalf("hits %+v, want the proposal labelled", retrieval.Hits)
	}
}

func TestAnythingTheReaderCannotSeeIsDroppedEvenIfTheStoreReturnedIt(t *testing.T) {
	h := newHarness(t)
	h.noProvider()

	project := uuid.New()
	leaked := h.memory("Billing freezes on the 1st", entity.ProjectAudience(project), entity.MemoryStatusAccepted)
	h.lexical = memories(leaked)

	retrieval, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{Query: "billing"})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if !retrieval.Empty() {
		t.Fatalf("hits %v, want none: the reader is not on the project", hitIDs(retrieval))
	}
}

func TestAnEmptyRetrievalStatesTheScopeThatWasSearched(t *testing.T) {
	h := newHarness(t)
	h.noProvider()

	project := uuid.New()

	retrieval, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{
		Query:     "who owns the moon",
		ProjectID: &project,
		Component: " api ",
	})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	scope := retrieval.Scope
	if !retrieval.Empty() || scope.ProjectID == nil || *scope.ProjectID != project || scope.Component != "api" ||
		!slices.Equal(scope.Kinds, entity.KnowledgeSubjectKinds()) || scope.Historical {
		t.Fatalf("retrieval %+v, want an explicit empty result naming what was searched", retrieval)
	}
}

func TestAHistoricalQuestionReadsWhatWasInEffectThen(t *testing.T) {
	h := newHarness(t)

	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	then := entity.Memory{
		ID: uuid.New(), WorkspaceID: h.workspaceID, Statement: "Deploy with Capistrano",
		Audience: entity.WorkspaceAudience(), Status: entity.MemoryStatusAccepted,
	}

	h.revisions.EXPECT().InEffect(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, query repository.KnowledgeQuery) ([]entity.KnowledgeCandidate, error) {
			if !query.On.Equal(at) {
				t.Errorf("asked about %v, want %v", query.On, at)
			}

			return []entity.KnowledgeCandidate{{Kind: entity.KnowledgeSubjectMemory, ID: then.ID, Memory: &then}}, nil
		})

	retrieval, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{
		Query: "how do we deploy", AsOf: &at,
	})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if retrieval.Semantic != entity.KnowledgeSemanticHistorical || !retrieval.Scope.Historical ||
		len(retrieval.Hits) != 1 || retrieval.Hits[0].Memory.Statement != "Deploy with Capistrano" {
		t.Fatalf("retrieval %+v", retrieval)
	}

	docsOnly, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{
		Query: "how do we deploy", AsOf: &at, Kinds: []entity.KnowledgeSubjectKind{entity.KnowledgeSubjectDocPassage},
	})
	if err != nil || !docsOnly.Empty() || len(docsOnly.Scope.Kinds) != 0 {
		t.Fatalf("docs as of a date = %+v, %v; want an empty result that searched nothing", docsOnly, err)
	}
}

func TestEnumerationPagesWithALookaheadAndNeverRanks(t *testing.T) {
	h := newHarness(t)

	var ids []uuid.UUID
	for range 3 {
		ids = append(ids, h.memory("Constraint", entity.WorkspaceAudience(), entity.MemoryStatusAccepted))
	}

	var filters []repository.MemoryFilter

	h.memories.EXPECT().Enumerate(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any(), 3).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, filter repository.MemoryFilter, after *uuid.UUID, _ int) ([]entity.Memory, error) {
			filters = append(filters, filter)

			if after != nil {
				return []entity.Memory{h.stored[ids[2]]}, nil
			}

			return []entity.Memory{h.stored[ids[0]], h.stored[ids[1]], h.stored[ids[2]]}, nil
		}).
		Times(2)

	first, err := h.service.Enumerate(context.Background(), h.workspaceID, service.EnumerateInput{
		MemoryKinds: []entity.MemoryKind{entity.MemoryKindConstraint}, Limit: 2,
	})
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}

	if len(first.Memories) != 2 || first.NextCursor == "" {
		t.Fatalf("first page %d memories, cursor %q", len(first.Memories), first.NextCursor)
	}

	second, err := h.service.Enumerate(context.Background(), h.workspaceID, service.EnumerateInput{
		MemoryKinds: []entity.MemoryKind{entity.MemoryKindConstraint}, Limit: 2, Cursor: first.NextCursor,
	})
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}

	if len(second.Memories) != 1 || second.NextCursor != "" || second.Memories[0].ID != ids[2] {
		t.Fatalf("second page %+v", second)
	}

	filter := filters[0]
	if !filter.WorkspaceWide || filter.InEffectOn == nil ||
		!slices.Equal(filter.Statuses, []entity.MemoryStatus{entity.MemoryStatusAccepted}) {
		t.Fatalf("filter %+v, want accepted guidance in effect today, workspace-wide included", filter)
	}

	if !summarized(first.Memories[0].Evidence) {
		t.Fatalf("enumeration handed back evidence %+v, want only each source's kind and state", first.Memories[0].Evidence)
	}

	if _, err := h.service.Enumerate(context.Background(), h.workspaceID, service.EnumerateInput{Cursor: "%%"}); !errors.Is(err, entity.ErrKnowledgeCursorInvalid) {
		t.Fatalf("garbage cursor: err = %v", err)
	}
}

func TestRecallFlagsChangedSourcesAndConcealsOnesTheReaderCannotRead(t *testing.T) {
	h := newHarness(t)
	h.noProvider()

	readable, hidden := uuid.New(), uuid.New()
	h.visible = []uuid.UUID{readable}

	id := h.memory("Ship on Tuesdays", entity.WorkspaceAudience(), entity.MemoryStatusAccepted)
	memory := h.stored[id]
	memory.Evidence = []entity.MemoryEvidence{
		{Kind: entity.MemoryEvidenceIssue, IssueID: &readable, State: entity.MemoryEvidenceChanged},
		{Kind: entity.MemoryEvidenceIssue, IssueID: &hidden, State: entity.MemoryEvidenceCurrent},
	}
	h.stored[id] = memory
	h.lexical = memories(id)

	retrieval, err := h.service.Retrieve(context.Background(), h.workspaceID, service.RetrieveInput{Query: "ship"})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	want := []entity.MemoryEvidence{
		{Kind: entity.MemoryEvidenceIssue, State: entity.MemoryEvidenceChanged},
		{Kind: entity.MemoryEvidenceIssue, State: entity.MemoryEvidenceUnavailable},
	}

	if len(retrieval.Hits) != 1 || !slices.Equal(retrieval.Hits[0].Memory.Evidence, want) {
		t.Fatalf("recall answered %+v, want %+v", retrieval.Hits, want)
	}
}

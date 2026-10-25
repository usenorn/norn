package knowledgeindex_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	aimodelrepo "github.com/usenorn/norn/internal/repository/aimodel"
	aiproviderrepo "github.com/usenorn/norn/internal/repository/aiprovider"
	docrepo "github.com/usenorn/norn/internal/repository/doc"
	docpassagerepo "github.com/usenorn/norn/internal/repository/docpassage"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	knowledgerepo "github.com/usenorn/norn/internal/repository/knowledge"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	knowledgeindexsvc "github.com/usenorn/norn/internal/service/knowledgeindex"
)

type harness struct {
	docs       *docrepo.MockDoc
	passages   *docpassagerepo.MockDocPassage
	embeddings *knowledgerepo.MockKnowledgeEmbedding
	providers  *aiproviderrepo.MockAIProvider
	models     *aimodelrepo.MockAIModel
	jobs       *jobqueuerepo.MockJobProducer
	service    service.KnowledgeIndexes

	workspaceID uuid.UUID
	outdated    []uuid.UUID
	replaced    []uuid.UUID
	stale       []entity.KnowledgeText
	saved       map[string][]float32
	batches     []int
	pruned      []string
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		docs:        docrepo.NewMockDoc(ctrl),
		passages:    docpassagerepo.NewMockDocPassage(ctrl),
		embeddings:  knowledgerepo.NewMockKnowledgeEmbedding(ctrl),
		providers:   aiproviderrepo.NewMockAIProvider(ctrl),
		models:      aimodelrepo.NewMockAIModel(ctrl),
		jobs:        jobqueuerepo.NewMockJobProducer(ctrl),
		workspaceID: uuid.New(),
		saved:       map[string][]float32{},
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()

	h.passages.EXPECT().Outdated(gomock.Any(), h.workspaceID, entity.KnowledgePassageBatchSize).
		DoAndReturn(func(context.Context, uuid.UUID, int) ([]uuid.UUID, error) {
			outdated := h.outdated
			h.outdated = nil

			return outdated, nil
		}).
		AnyTimes()
	h.docs.EXPECT().LockByID(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID, docID uuid.UUID) (entity.Doc, error) {
			return entity.Doc{DocSummary: entity.DocSummary{ID: docID, WorkspaceID: workspaceID, Version: 3}}, nil
		}).
		AnyTimes()
	h.passages.EXPECT().Replace(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, doc entity.Doc) error {
			h.replaced = append(h.replaced, doc.ID)

			return nil
		}).
		AnyTimes()
	h.embeddings.EXPECT().Stale(gomock.Any(), h.workspaceID, gomock.Any(), entity.KnowledgeStaleBatchSize).
		DoAndReturn(func(context.Context, uuid.UUID, string, int) ([]entity.KnowledgeText, error) {
			var stale []entity.KnowledgeText

			for _, text := range h.stale {
				if _, done := h.saved[string(text.ContentHash)]; !done {
					stale = append(stale, text)
				}
			}

			return stale, nil
		}).
		AnyTimes()
	h.embeddings.EXPECT().Save(gomock.Any(), h.workspaceID, "text-embedding-3-small", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, _ string, embeddings []entity.KnowledgeEmbedding) error {
			for _, embedding := range embeddings {
				h.saved[string(embedding.ContentHash)] = embedding.Vector
			}

			return nil
		}).
		AnyTimes()
	h.embeddings.EXPECT().Prune(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, model string) error {
			h.pruned = append(h.pruned, model)

			return nil
		}).
		AnyTimes()
	h.models.EXPECT().Embed(gomock.Any(), gomock.Any(), "sk-live", "text-embedding-3-small", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.AIProviderEndpoint, _, _ string, inputs []string) ([][]float32, error) {
			h.batches = append(h.batches, len(inputs))

			vectors := make([][]float32, len(inputs))
			for index, input := range inputs {
				vectors[index] = []float32{float32(len(input))}
			}

			return vectors, nil
		}).
		AnyTimes()

	h.service = knowledgeindexsvc.New(h.docs, h.passages, h.embeddings, h.providers, h.models, h.jobs, transactor)

	return h
}

func (h *harness) provider(embeddingModel string) {
	h.providers.EXPECT().Get(gomock.Any(), h.workspaceID).
		Return(entity.AIProviderConnection{WorkspaceID: h.workspaceID, EmbeddingModel: embeddingModel}, nil).
		AnyTimes()
	h.providers.EXPECT().APIKey(gomock.Any(), h.workspaceID).Return("sk-live", nil).AnyTimes()
}

func (h *harness) texts(count int) {
	for index := range count {
		h.stale = append(h.stale, entity.KnowledgeText{
			ContentHash: []byte(fmt.Sprintf("hash-%03d", index)),
			Text:        fmt.Sprintf("text %d", index),
		})
	}
}

func TestWithoutAProviderTheRefreshKeepsPassagesAndDropsEveryEmbedding(t *testing.T) {
	h := newHarness(t)
	h.providers.EXPECT().Get(gomock.Any(), h.workspaceID).
		Return(entity.AIProviderConnection{}, entity.ErrAIProviderNotConfigured)
	h.outdated = []uuid.UUID{uuid.New(), uuid.New()}
	want := slices.Clone(h.outdated)
	h.texts(3)

	if err := h.service.Refresh(context.Background(), h.workspaceID); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if !slices.Equal(h.replaced, want) {
		t.Errorf("rebuilt passages for %v, want %v: word matching needs them with or without a provider", h.replaced, want)
	}

	if len(h.batches) != 0 || !slices.Equal(h.pruned, []string{""}) {
		t.Errorf("embedded %v and pruned %q, want no embedding and every stored vector dropped", h.batches, h.pruned)
	}
}

func TestAProviderWithoutAnEmbeddingModelEmbedsNothing(t *testing.T) {
	h := newHarness(t)
	h.provider("")
	h.texts(3)

	if err := h.service.Refresh(context.Background(), h.workspaceID); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if len(h.batches) != 0 || !slices.Equal(h.pruned, []string{""}) {
		t.Errorf("embedded %v and pruned %q, want nothing embedded and every stored vector dropped", h.batches, h.pruned)
	}
}

func TestEveryStaleTextIsEmbeddedInBatchesBesideItsOwnHash(t *testing.T) {
	h := newHarness(t)
	h.provider("text-embedding-3-small")
	h.texts(entity.KnowledgeEmbedBatchSize + 6)

	if err := h.service.Refresh(context.Background(), h.workspaceID); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if !slices.Equal(h.batches, []int{entity.KnowledgeEmbedBatchSize, 6}) {
		t.Errorf("batches = %v, want one full batch and the remainder", h.batches)
	}

	for _, text := range h.stale {
		if vector := h.saved[string(text.ContentHash)]; len(vector) != 1 || vector[0] != float32(len(text.Text)) {
			t.Fatalf("hash %s saved %v, want the vector of its own text", text.ContentHash, vector)
		}
	}

	if !slices.Equal(h.pruned, []string{"text-embedding-3-small"}) {
		t.Errorf("pruned %q, want vectors from any other model dropped", h.pruned)
	}
}

func TestATextThatStaysStaleStopsTheRefreshInsteadOfLooping(t *testing.T) {
	h := newHarness(t)
	h.provider("text-embedding-3-small")

	stuck := entity.KnowledgeText{ContentHash: []byte("stuck"), Text: "never saved"}

	ctrl := gomock.NewController(t)
	embeddings := knowledgerepo.NewMockKnowledgeEmbedding(ctrl)
	embeddings.EXPECT().Stale(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		Return([]entity.KnowledgeText{stuck}, nil).Times(2)
	embeddings.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	transactor := transactorrepo.NewMockTransactor(ctrl)
	refresh := knowledgeindexsvc.New(h.docs, h.passages, embeddings, h.providers, h.models, h.jobs, transactor)

	if err := refresh.Refresh(context.Background(), h.workspaceID); err == nil {
		t.Fatal("a text that never leaves the stale list was embedded again and again")
	}

	if len(h.batches) != 1 {
		t.Fatalf("called the provider %d times for one stuck text, want once", len(h.batches))
	}
}

func TestTheSweepWakesEveryWorkspaceWithKnowledgeAwaitingRefresh(t *testing.T) {
	h := newHarness(t)
	pending := []uuid.UUID{uuid.New(), uuid.New()}

	h.embeddings.EXPECT().Pending(gomock.Any(), entity.KnowledgeSweepWorkspaceLimit).Return(pending, nil)

	var woken []uuid.UUID

	h.jobs.EXPECT().EnqueueKnowledgeRefresh(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload entity.KnowledgeRefreshPayload) error {
			woken = append(woken, payload.WorkspaceID)

			return nil
		}).
		Times(len(pending))

	if err := h.service.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if !slices.Equal(woken, pending) {
		t.Fatalf("woke %v, want %v", woken, pending)
	}
}

func TestARefreshCarriesAProviderRefusalOut(t *testing.T) {
	h := newHarness(t)
	h.providers.EXPECT().Get(gomock.Any(), h.workspaceID).
		Return(entity.AIProviderConnection{WorkspaceID: h.workspaceID, EmbeddingModel: "text-embedding-3-small"}, nil)
	h.providers.EXPECT().APIKey(gomock.Any(), h.workspaceID).Return("sk-revoked", nil)
	h.models.EXPECT().Embed(gomock.Any(), gomock.Any(), "sk-revoked", gomock.Any(), gomock.Any()).
		Return(nil, entity.ErrAIProviderKeyRejected)
	h.texts(1)

	if err := h.service.Refresh(context.Background(), h.workspaceID); !errors.Is(err, entity.ErrAIProviderKeyRejected) {
		t.Fatalf("err = %v, want ErrAIProviderKeyRejected so the job can tell a refusal from an outage", err)
	}

	if len(h.pruned) != 0 {
		t.Errorf("pruned %q after a refusal; stored vectors must survive a provider that is briefly unusable", h.pruned)
	}
}

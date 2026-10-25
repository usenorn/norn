package knowledgeembedding_test

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
	aimodelrepo "github.com/usenorn/norn/internal/repository/aimodel"
	aiproviderrepo "github.com/usenorn/norn/internal/repository/aiprovider"
	knowledgeembeddingsvc "github.com/usenorn/norn/internal/service/knowledgeembedding"
)

func TestATextIsEmbeddedOnlyWhenTheWorkspaceHasAnEmbeddingModel(t *testing.T) {
	workspaceID := uuid.New()
	embedding := entity.AIProviderConnection{WorkspaceID: workspaceID, EmbeddingModel: "text-embedding-3-small"}

	for name, testCase := range map[string]struct {
		connection entity.AIProviderConnection
		getErr     error
		embedErr   error
		want       entity.KnowledgeSemantic
	}{
		"no provider":                 {getErr: entity.ErrAIProviderNotConfigured, want: entity.KnowledgeSemanticUnconfigured},
		"a provider with no embedder": {connection: entity.AIProviderConnection{WorkspaceID: workspaceID}, want: entity.KnowledgeSemanticUnconfigured},
		"a failing embedder":          {connection: embedding, embedErr: entity.ErrAIProviderRateLimited, want: entity.KnowledgeSemanticUnavailable},
		"a working embedder":          {connection: embedding, want: entity.KnowledgeSemanticUsed},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			providers := aiproviderrepo.NewMockAIProvider(ctrl)
			models := aimodelrepo.NewMockAIModel(ctrl)

			providers.EXPECT().Get(gomock.Any(), workspaceID).Return(testCase.connection, testCase.getErr)
			providers.EXPECT().APIKey(gomock.Any(), workspaceID).Return("sk-live", nil).AnyTimes()
			models.EXPECT().Embed(gomock.Any(), gomock.Any(), "sk-live", "text-embedding-3-small", []string{"ship on tuesdays"}).
				DoAndReturn(func(context.Context, entity.AIProviderEndpoint, string, string, []string) ([][]float32, error) {
					if testCase.embedErr != nil {
						return nil, testCase.embedErr
					}

					return [][]float32{{0.6, 0.8}}, nil
				}).
				AnyTimes()

			embeddings := knowledgeembeddingsvc.New(providers, models, config.Knowledge{QueryTimeout: time.Second})

			vector, state, err := embeddings.Embed(context.Background(), workspaceID, "  ship on tuesdays ")
			if err != nil {
				t.Fatalf("embedding: %v", err)
			}

			if state != testCase.want {
				t.Fatalf("state = %s, want %s", state, testCase.want)
			}

			wantVector := state == entity.KnowledgeSemanticUsed
			if got := vector.Model == "text-embedding-3-small" && slices.Equal(vector.Vector, []float32{0.6, 0.8}); got != wantVector {
				t.Fatalf("vector = %+v, want one only when used", vector)
			}
		})
	}
}

func TestAProviderLookupFailureIsAnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	providers := aiproviderrepo.NewMockAIProvider(ctrl)
	broken := errors.New("connection refused")

	providers.EXPECT().Get(gomock.Any(), gomock.Any()).Return(entity.AIProviderConnection{}, broken)

	embeddings := knowledgeembeddingsvc.New(providers, aimodelrepo.NewMockAIModel(ctrl), config.Knowledge{QueryTimeout: time.Second})

	if _, _, err := embeddings.Embed(context.Background(), uuid.New(), "anything"); !errors.Is(err, broken) {
		t.Fatalf("got %v, want the lookup failure", err)
	}
}

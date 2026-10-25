package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=ai_model.go -destination=aimodel/mock_ai_model.go -package=aimodel -mock_names=AIModel=MockAIModel

type AIModel interface {
	List(ctx context.Context, endpoint entity.AIProviderEndpoint, apiKey string) (entity.AIProviderModels, error)
	Embed(
		ctx context.Context,
		endpoint entity.AIProviderEndpoint,
		apiKey, model string,
		inputs []string,
	) ([][]float32, error)
	Probe(ctx context.Context, endpoint entity.AIProviderEndpoint, apiKey, model string) error
	Converse(
		ctx context.Context,
		endpoint entity.AIProviderEndpoint,
		apiKey string,
		request entity.AIConversationRequest,
	) (entity.AIConversationStep, error)
}

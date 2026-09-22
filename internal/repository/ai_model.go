package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=ai_model.go -destination=aimodel/mock_ai_model.go -package=aimodel -mock_names=AIModel=MockAIModel

type AIModel interface {
	List(ctx context.Context, endpoint entity.AIProviderEndpoint, apiKey string) ([]string, error)
	Probe(ctx context.Context, endpoint entity.AIProviderEndpoint, apiKey, model string) error
}

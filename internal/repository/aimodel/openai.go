package aimodel

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/openai"
	"github.com/usenorn/norn/internal/repository"
)

type openAIModels struct {
	client *openai.Client
}

func New(client *openai.Client) repository.AIModel {
	return &openAIModels{client: client}
}

func (r *openAIModels) List(
	ctx context.Context,
	endpoint entity.AIProviderEndpoint,
	apiKey string,
) ([]string, error) {
	return r.client.TextModels(ctx, endpoint, apiKey)
}

func (r *openAIModels) Probe(
	ctx context.Context,
	endpoint entity.AIProviderEndpoint,
	apiKey, model string,
) error {
	return r.client.Probe(ctx, endpoint, apiKey, model)
}

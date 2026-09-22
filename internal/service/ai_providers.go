package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=ai_providers.go -destination=aiprovider/mock_ai_providers.go -package=aiprovider -mock_names=AIProviders=MockAIProviders

type AIProviders interface {
	Get(ctx context.Context, workspaceID uuid.UUID) (entity.AIProviderConnection, error)
	Models(ctx context.Context, workspaceID uuid.UUID) ([]string, error)
	Configure(ctx context.Context, workspaceID uuid.UUID, input entity.AIProviderInput) (entity.AIProviderConnection, error)
	Test(ctx context.Context, workspaceID uuid.UUID) (entity.AIProviderConnection, error)
	Remove(ctx context.Context, workspaceID uuid.UUID) error
}

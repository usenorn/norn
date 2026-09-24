package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=ai_provider.go -destination=aiprovider/mock_ai_provider.go -package=aiprovider -mock_names=AIProvider=MockAIProvider

type AIProvider interface {
	Get(ctx context.Context, workspaceID uuid.UUID) (entity.AIProviderConnection, error)
	APIKey(ctx context.Context, workspaceID uuid.UUID) (string, error)
	Save(ctx context.Context, connection entity.AIProviderConnection, apiKey string) (entity.AIProviderConnection, error)
	Update(ctx context.Context, connection entity.AIProviderConnection) (entity.AIProviderConnection, error)
	MarkVerified(ctx context.Context, workspaceID uuid.UUID, at time.Time) error
	MarkFailed(ctx context.Context, workspaceID uuid.UUID, failure entity.AIProviderFailure, at time.Time) error
	Delete(ctx context.Context, workspaceID uuid.UUID) error
}

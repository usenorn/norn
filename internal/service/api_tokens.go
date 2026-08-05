package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=api_tokens.go -destination=apitoken/mock_api_tokens.go -package=apitoken -mock_names=APITokens=MockAPITokens

type APITokens interface {
	Mint(ctx context.Context, input MintAPITokenInput) (MintedAPIToken, error)
	List(ctx context.Context) ([]entity.APIToken, error)
	ListForWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]OwnedAPIToken, error)
	Revoke(ctx context.Context, tokenID uuid.UUID) error
	RevokeInWorkspace(ctx context.Context, workspaceID, tokenID uuid.UUID) error
	Authenticate(ctx context.Context, token string) (entity.Actor, error)
	SweepExpiring(ctx context.Context) error
}

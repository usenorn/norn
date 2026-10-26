package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workspace_domains.go -destination=workspacedomain/mock_workspace_domains.go -package=workspacedomain -mock_names=WorkspaceDomains=MockWorkspaceDomains

type WorkspaceDomains interface {
	List(ctx context.Context, workspaceID uuid.UUID) ([]entity.WorkspaceDomain, error)
	Add(ctx context.Context, workspaceID uuid.UUID, domain string) (entity.WorkspaceDomain, error)
	Verify(ctx context.Context, workspaceID, id uuid.UUID) (entity.WorkspaceDomain, error)
	Remove(ctx context.Context, workspaceID, id uuid.UUID) error
}

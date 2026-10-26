package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workspace_domain.go -destination=workspacedomain/mock_workspace_domain.go -package=workspacedomain -mock_names=WorkspaceDomain=MockWorkspaceDomain

type WorkspaceDomain interface {
	List(ctx context.Context, workspaceID uuid.UUID) ([]entity.WorkspaceDomain, error)
	Get(ctx context.Context, workspaceID, id uuid.UUID) (entity.WorkspaceDomain, error)
	Add(ctx context.Context, domain entity.WorkspaceDomain) (entity.WorkspaceDomain, error)
	MarkChecked(ctx context.Context, workspaceID, id uuid.UUID, verified bool, at time.Time) (entity.WorkspaceDomain, error)
	Delete(ctx context.Context, workspaceID, id uuid.UUID) error
	VerifiedWorkspaceIDs(ctx context.Context, domain string) ([]uuid.UUID, error)
}

package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workspace.go -destination=workspace/mock_workspace.go -package=workspace -mock_names=Workspace=MockWorkspace

type Workspace interface {
	Create(ctx context.Context, workspace entity.Workspace) (entity.Workspace, error)
	GetByID(ctx context.Context, id uuid.UUID) (entity.Workspace, error)
	ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]entity.Workspace, error)
	LockByIDs(ctx context.Context, ids []uuid.UUID) error
}

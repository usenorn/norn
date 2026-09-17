package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workspace.go -destination=workspace/mock_workspace.go -package=workspace -mock_names=Workspace=MockWorkspace

type Workspace interface {
	Create(ctx context.Context, workspace entity.Workspace) (entity.Workspace, error)
	GetByID(ctx context.Context, id uuid.UUID) (entity.Workspace, error)
	GetBySlug(ctx context.Context, slug string) (entity.Workspace, error)
	ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]entity.Workspace, error)
	LockByIDs(ctx context.Context, ids []uuid.UUID) error
	UpdateSettings(ctx context.Context, id uuid.UUID, settings WorkspaceSettings) (entity.Workspace, error)
	SetLogo(ctx context.Context, id uuid.UUID, objectKey string) (entity.Workspace, error)
	ReserveSlug(ctx context.Context, slug string, claimant uuid.UUID, now time.Time) error
	RecordSlugRedirect(ctx context.Context, slug string, workspaceID uuid.UUID, expiresAt time.Time) error
	ResolveSlugRedirect(ctx context.Context, slug string, now time.Time) (uuid.UUID, error)
	MarkPendingDeletion(ctx context.Context, id uuid.UUID, requestedAt, purgeAfter time.Time) (entity.Workspace, error)
	Restore(ctx context.Context, id uuid.UUID) (entity.Workspace, error)
	Purge(ctx context.Context, id uuid.UUID) error
}

type WorkspaceSettings struct {
	Slug          string
	Name          string
	Timezone      string
	WeekStartsOn  entity.WeekDay
	DefaultTeamID *uuid.UUID
}

package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=saved_view.go -destination=savedview/mock_saved_view.go -package=savedview -mock_names=SavedView=MockSavedView

type SavedViewSettings struct {
	Name    string
	Sharing entity.SavedViewSharing
	TeamID  uuid.UUID
	Filter  entity.IssueFilter
	Sort    []entity.IssueSort
	GroupBy entity.IssueGroupBy
}

type SavedView interface {
	Create(ctx context.Context, view entity.SavedView) (entity.SavedView, error)
	GetByID(ctx context.Context, workspaceID, savedViewID uuid.UUID) (entity.SavedView, error)
	LockByID(ctx context.Context, workspaceID, savedViewID uuid.UUID) (entity.SavedView, error)
	ListFor(ctx context.Context, workspaceID, accountID uuid.UUID, teamIDs []uuid.UUID) ([]entity.SavedView, error)
	UpdateSettings(ctx context.Context, savedViewID uuid.UUID, settings SavedViewSettings) (entity.SavedView, error)
	Delete(ctx context.Context, savedViewID uuid.UUID) error
	Place(ctx context.Context, workspaceID, accountID uuid.UUID, orderedIDs []uuid.UUID) error
}

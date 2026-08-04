package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=saved_views.go -destination=savedview/mock_saved_views.go -package=savedview -mock_names=SavedViews=MockSavedViews

type CreateSavedViewInput struct {
	WorkspaceID uuid.UUID
	Name        string
	Sharing     entity.SavedViewSharing
	TeamID      uuid.UUID
	Filter      *entity.IssueFilter
	Sort        []entity.IssueSort
	GroupBy     entity.IssueGroupBy
}

type UpdateSavedViewInput struct {
	Name    *string
	Sharing *entity.SavedViewSharing
	TeamID  *uuid.UUID
	Filter  *entity.IssueFilter
	Sort    *[]entity.IssueSort
	GroupBy *entity.IssueGroupBy
}

type SavedViewSummary struct {
	View     entity.SavedView
	Editable bool
}

type SavedViewDetail struct {
	Summary    SavedViewSummary
	References []entity.IssueFilterReference
}

type SavedViews interface {
	List(ctx context.Context, workspaceID uuid.UUID) ([]SavedViewSummary, error)
	Get(ctx context.Context, workspaceID, savedViewID uuid.UUID) (SavedViewDetail, error)
	Create(ctx context.Context, input CreateSavedViewInput) (SavedViewSummary, error)
	Update(ctx context.Context, workspaceID, savedViewID uuid.UUID, input UpdateSavedViewInput) (SavedViewSummary, error)
	Remove(ctx context.Context, workspaceID, savedViewID uuid.UUID, acknowledgedSharing entity.SavedViewSharing) error
	Reorder(ctx context.Context, workspaceID uuid.UUID, orderedIDs []uuid.UUID) ([]SavedViewSummary, error)
}

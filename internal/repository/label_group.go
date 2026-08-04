package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=label_group.go -destination=labelgroup/mock_label_group.go -package=labelgroup -mock_names=LabelGroup=MockLabelGroup

type LabelGroup interface {
	Create(ctx context.Context, group entity.LabelGroup) (entity.LabelGroup, error)
	GetByID(ctx context.Context, workspaceID, id uuid.UUID) (entity.LabelGroup, error)
	ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]entity.LabelGroup, error)
	UpdateName(ctx context.Context, id uuid.UUID, name string) (entity.LabelGroup, error)
	Ungroup(ctx context.Context, groupID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

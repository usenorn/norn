package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=labels.go -destination=label/mock_labels.go -package=label -mock_names=Labels=MockLabels

type Labels interface {
	List(ctx context.Context, workspaceID uuid.UUID) ([]entity.Label, error)
	Create(ctx context.Context, input CreateLabelInput) (entity.Label, error)
	Update(ctx context.Context, workspaceID, labelID uuid.UUID, input UpdateLabelInput) (entity.Label, error)
	Merge(ctx context.Context, workspaceID, sourceID, targetID uuid.UUID) (entity.Label, error)
	Usage(ctx context.Context, workspaceID, labelID uuid.UUID) (entity.LabelUsage, error)
	Remove(ctx context.Context, workspaceID, labelID uuid.UUID, acknowledgedIssues int) error
	Groups(ctx context.Context, workspaceID uuid.UUID) ([]entity.LabelGroup, error)
	CreateGroup(ctx context.Context, workspaceID uuid.UUID, name string) (entity.LabelGroup, error)
	RenameGroup(ctx context.Context, workspaceID, groupID uuid.UUID, name string) (entity.LabelGroup, error)
	RemoveGroup(ctx context.Context, workspaceID, groupID uuid.UUID) error
}

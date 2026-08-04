package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=label.go -destination=label/mock_label.go -package=label -mock_names=Label=MockLabel

type Label interface {
	Create(ctx context.Context, label entity.Label) (entity.Label, error)
	GetByID(ctx context.Context, workspaceID, id uuid.UUID) (entity.Label, error)
	LockByID(ctx context.Context, workspaceID, id uuid.UUID) (entity.Label, error)
	ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, scope entity.TeamScope) ([]entity.Label, error)
	ListByIDs(ctx context.Context, workspaceID uuid.UUID, ids []uuid.UUID) ([]entity.Label, error)
	UpdateSettings(ctx context.Context, id uuid.UUID, name string, color entity.LabelColor, groupID uuid.UUID) (entity.Label, error)
	SyncApplicationGroup(ctx context.Context, labelID, groupID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	SetForIssue(ctx context.Context, issue entity.Issue, labels []entity.Label) error
	Usage(ctx context.Context, labelID uuid.UUID, scope entity.TeamScope) (entity.LabelUsage, error)
	MoveApplications(ctx context.Context, source, target entity.Label) error
}

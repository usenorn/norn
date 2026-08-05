package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=activity.go -destination=activity/mock_activity.go -package=activity -mock_names=Activity=MockActivity

type Activity interface {
	Record(ctx context.Context, activity entity.Activity) error
	ListBySubject(ctx context.Context, subject entity.ActivitySubject, page entity.ActivityPage) ([]entity.ActivityEvent, error)
	ListByActor(ctx context.Context, workspaceID, accountID uuid.UUID, page entity.ActivityPage) ([]entity.ActivityEvent, error)
}

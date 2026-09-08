package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=activity.go -destination=activity/mock_activity.go -package=activity -mock_names=Activity=MockActivity

type Activity interface {
	Record(ctx context.Context, activity entity.Activity) error
	ListStateChanges(ctx context.Context, issueIDs []uuid.UUID, since time.Time) ([]entity.CycleStateChange, error)
	ListBySubject(ctx context.Context, subject entity.ActivitySubject, page entity.ActivityPage) ([]entity.ActivityEvent, error)
	ListByActor(ctx context.Context, workspaceID, accountID uuid.UUID, page entity.ActivityPage) ([]entity.ActivityEvent, error)
}

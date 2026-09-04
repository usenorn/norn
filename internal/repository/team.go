package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=team.go -destination=team/mock_team.go -package=team -mock_names=Team=MockTeam

type Team interface {
	Create(ctx context.Context, team entity.Team) (entity.Team, error)
	GetByID(ctx context.Context, id uuid.UUID) (entity.Team, error)
	ListVisibleTo(ctx context.Context, workspaceID, accountID uuid.UUID, status entity.TeamStatus, includePrivate bool) ([]entity.Team, error)
	ListByWorkspaceMember(ctx context.Context, workspaceID, accountID uuid.UUID) ([]entity.Team, error)
	UpdateSettings(ctx context.Context, id uuid.UUID, settings TeamSettings) (entity.Team, error)
	Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) (entity.Team, error)
	Unarchive(ctx context.Context, id uuid.UUID) (entity.Team, error)
}

type TeamSettings struct {
	Name        string
	Description string
	Icon        string
	IconColor   entity.TeamColor
	Estimation  entity.TeamEstimation
	Visibility  entity.TeamVisibility
}

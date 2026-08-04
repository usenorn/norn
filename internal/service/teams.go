package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=teams.go -destination=team/mock_teams.go -package=team -mock_names=Teams=MockTeams

type Teams interface {
	Create(ctx context.Context, input CreateTeamInput) (entity.Team, error)
	Get(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.Team, error)
	List(ctx context.Context, workspaceID uuid.UUID, status entity.TeamStatus) ([]entity.Team, error)
	Update(ctx context.Context, workspaceID, teamID uuid.UUID, input UpdateTeamInput) (entity.Team, error)
	Archive(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.Team, error)
	Unarchive(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.Team, error)
	ListMembers(ctx context.Context, workspaceID, teamID uuid.UUID) ([]TeamMemberView, error)
	AddMember(ctx context.Context, workspaceID, teamID, accountID uuid.UUID) (TeamMemberView, error)
	RemoveMember(ctx context.Context, workspaceID, teamID, accountID uuid.UUID) error
}

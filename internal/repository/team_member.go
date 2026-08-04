package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=team_member.go -destination=teammember/mock_team_member.go -package=teammember -mock_names=TeamMember=MockTeamMember

type TeamMember interface {
	Create(ctx context.Context, membership entity.TeamMembership) (entity.TeamMembership, error)
	Get(ctx context.Context, teamID, accountID uuid.UUID) (entity.TeamMembership, error)
	ListByTeamID(ctx context.Context, teamID uuid.UUID) ([]entity.TeamMembership, error)
	Delete(ctx context.Context, teamID, accountID uuid.UUID) error
}

package service

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type CreateTeamInput struct {
	WorkspaceID uuid.UUID
	Key         string
	Name        string
	Visibility  entity.TeamVisibility
}

type UpdateTeamInput struct {
	Name        *string
	Description *string
	Icon        *string
	IconColor   *entity.TeamColor
	Estimation  *entity.TeamEstimation
	Visibility  *entity.TeamVisibility
}

type TeamMemberView struct {
	Membership  entity.TeamMembership
	DisplayName string
	Email       string
}

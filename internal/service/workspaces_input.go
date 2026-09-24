package service

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type CreateWorkspaceInput struct {
	Slug string
	Name string
	Team *CreateWorkspaceTeamInput
}

type CreateWorkspaceTeamInput struct {
	Key  string
	Name string
}

type UpdateWorkspaceInput struct {
	Slug              *string
	Name              *string
	Timezone          *string
	WeekStartsOn      *entity.WeekDay
	AgentInstructions *string
	DefaultTeamID     *uuid.UUID
}

type ListMembersInput struct {
	Query  string
	Kinds  []entity.AccountKind
	Cursor string
	Limit  int
}

type MemberPage struct {
	Members    []entity.WorkspaceMember
	NextCursor string
}

type MemberRemoval struct {
	Member           entity.WorkspaceMember
	Teams            []entity.Team
	SoleAdmin        bool
	DirectoryManaged bool
}

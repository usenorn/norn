package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workspaces.go -destination=workspace/mock_workspaces.go -package=workspace -mock_names=Workspaces=MockWorkspaces

type Workspaces interface {
	Create(ctx context.Context, input CreateWorkspaceInput) (entity.Workspace, error)
	ListForAccount(ctx context.Context, accountID uuid.UUID) ([]entity.Workspace, error)
	ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]entity.Membership, error)
	AddMember(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.Membership, error)
	ChangeMemberRole(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.Membership, error)
	RemoveMember(ctx context.Context, workspaceID, accountID uuid.UUID) error
	AuthPolicy(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceAuthPolicy, error)
	SetAuthPolicy(ctx context.Context, workspaceID uuid.UUID, enforcement entity.AuthEnforcement) (entity.WorkspaceAuthPolicy, error)
}

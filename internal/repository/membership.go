package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=membership.go -destination=membership/mock_membership.go -package=membership -mock_names=Membership=MockMembership

type Membership interface {
	Create(ctx context.Context, membership entity.Membership) (entity.Membership, error)
	Get(ctx context.Context, workspaceID, accountID uuid.UUID) (entity.Membership, error)
	ListPageByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, page entity.MembershipPage) ([]entity.WorkspaceMember, error)
	UpdateRole(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.Membership, error)
	SetAuditAccess(ctx context.Context, workspaceID, accountID uuid.UUID, reads bool) (entity.Membership, error)
	RecordActivity(ctx context.Context, accountID uuid.UUID, activeAt time.Time, method entity.SessionAuthMethod) error
	Delete(ctx context.Context, workspaceID, accountID uuid.UUID) error
	DeleteByAccountID(ctx context.Context, accountID uuid.UUID) error
	ListAdminWorkspaceIDs(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error)
	ListWorkspaceIDsWithoutOtherActiveAdmin(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error)
}

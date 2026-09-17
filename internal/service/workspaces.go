package service

import (
	"context"
	"io"
	"net/netip"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workspaces.go -destination=workspace/mock_workspaces.go -package=workspace -mock_names=Workspaces=MockWorkspaces

type Workspaces interface {
	Create(ctx context.Context, input CreateWorkspaceInput) (entity.Workspace, error)
	Get(ctx context.Context, workspaceID uuid.UUID) (entity.Workspace, error)
	Update(ctx context.Context, workspaceID uuid.UUID, input UpdateWorkspaceInput) (entity.Workspace, error)
	Delete(ctx context.Context, workspaceID uuid.UUID) (entity.Workspace, error)
	Restore(ctx context.Context, workspaceID uuid.UUID) (entity.Workspace, error)
	UploadLogo(ctx context.Context, workspaceID uuid.UUID, body io.Reader) (entity.Workspace, error)
	RemoveLogo(ctx context.Context, workspaceID uuid.UUID) (entity.Workspace, error)
	LogoContent(ctx context.Context, workspaceID uuid.UUID) (string, error)
	ResolveSlugRedirect(ctx context.Context, slug string) (entity.Workspace, error)
	Purge(ctx context.Context, workspaceID uuid.UUID) error
	ListForAccount(ctx context.Context, accountID uuid.UUID) ([]entity.Workspace, error)
	ListMembers(ctx context.Context, workspaceID uuid.UUID, input ListMembersInput) (MemberPage, error)
	AddMember(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.WorkspaceMember, error)
	ChangeMemberRole(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.WorkspaceMember, error)
	PreviewMemberRemoval(ctx context.Context, workspaceID, accountID uuid.UUID) (MemberRemoval, error)
	RemoveMember(ctx context.Context, workspaceID, accountID uuid.UUID, reassignTo *uuid.UUID) error
	AuthPolicy(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceAuthPolicy, error)
	SetAuthPolicy(ctx context.Context, workspaceID uuid.UUID, enforcement entity.AuthEnforcement) (AuthPolicyOutcome, error)
	EnforcementReadiness(ctx context.Context, workspaceID uuid.UUID) (entity.EnforcementBlocker, error)
	RedeemRecoveryCode(ctx context.Context, input RedeemRecoveryCodeInput) error
	ListSSOIdentities(ctx context.Context, workspaceID uuid.UUID) ([]entity.SSOIdentity, error)
	UnlinkSSOIdentity(ctx context.Context, workspaceID, accountID uuid.UUID) error
}

type AuthPolicyOutcome struct {
	Policy        entity.WorkspaceAuthPolicy
	RecoveryCodes []string
}

type RedeemRecoveryCodeInput struct {
	WorkspaceSlug string
	Code          string
	From          string
	FromAddress   netip.Addr
}

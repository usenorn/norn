package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workspace_auth_policy.go -destination=workspaceauthpolicy/mock_workspace_auth_policy.go -package=workspaceauthpolicy -mock_names=WorkspaceAuthPolicy=MockWorkspaceAuthPolicy

type WorkspaceAuthPolicy interface {
	Get(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceAuthPolicy, error)
	Upsert(ctx context.Context, policy entity.WorkspaceAuthPolicy) (entity.WorkspaceAuthPolicy, error)
	ListEnforcementsByAccountID(ctx context.Context, accountID uuid.UUID) ([]entity.AuthEnforcement, error)
}

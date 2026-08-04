package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=ssoidentity.go -destination=ssoidentity/mock_ssoidentity.go -package=ssoidentity -mock_names=SSOIdentity=MockSSOIdentity

type SSOIdentity interface {
	Link(ctx context.Context, identity entity.SSOIdentity) error
	Get(ctx context.Context, workspaceID, accountID uuid.UUID) (entity.SSOIdentity, error)
	ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]entity.SSOIdentity, error)
	Unlink(ctx context.Context, workspaceID, accountID uuid.UUID) error
	AnyLinkedAdmin(ctx context.Context, workspaceID uuid.UUID) (bool, error)
}

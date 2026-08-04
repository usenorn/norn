package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=oidcconnection.go -destination=oidcconnection/mock_oidcconnection.go -package=oidcconnection -mock_names=OIDCConnection=MockOIDCConnection

type OIDCConnection interface {
	Get(ctx context.Context, workspaceID uuid.UUID) (entity.OIDCConnection, error)
	Save(ctx context.Context, connection entity.OIDCConnection) (entity.OIDCConnection, error)
	Delete(ctx context.Context, workspaceID uuid.UUID) error
	MarkVerified(ctx context.Context, workspaceID uuid.UUID, at time.Time) error
}

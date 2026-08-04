package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=ssoconnection.go -destination=ssoconnection/mock_ssoconnection.go -package=ssoconnection -mock_names=SSOConnection=MockSSOConnection

type SSOConnection interface {
	Protocol(ctx context.Context, workspaceID uuid.UUID) (entity.SSOProtocol, error)
	GetOIDC(ctx context.Context, workspaceID uuid.UUID) (entity.OIDCConnection, error)
	SaveOIDC(ctx context.Context, connection entity.OIDCConnection) (entity.OIDCConnection, error)
	GetSAML(ctx context.Context, workspaceID uuid.UUID) (entity.SAMLConnection, error)
	SaveSAML(ctx context.Context, connection entity.SAMLConnection) (entity.SAMLConnection, error)
	Delete(ctx context.Context, workspaceID uuid.UUID) error
	MarkVerified(ctx context.Context, workspaceID uuid.UUID, at time.Time) error
	Verified(ctx context.Context, workspaceID uuid.UUID) (bool, error)
	ListSAMLCertificates(ctx context.Context) ([]entity.SAMLConnection, error)
	RecordExpiryNotice(ctx context.Context, workspaceID uuid.UUID, days int) error
}

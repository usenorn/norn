package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=sso_connections.go -destination=ssoconnection/mock_sso_connections.go -package=ssoconnection -mock_names=SSOConnections=MockSSOConnections

type SSOConnections interface {
	Get(ctx context.Context, workspaceID uuid.UUID) (entity.OIDCConnection, error)
	Save(ctx context.Context, input SaveOIDCConnectionInput) (entity.OIDCConnection, error)
	Remove(ctx context.Context, workspaceID uuid.UUID) error
	Discover(ctx context.Context, workspaceID uuid.UUID, issuer string) (entity.OIDCEndpoints, error)
	BeginTest(ctx context.Context, workspaceID uuid.UUID) (string, error)
	BeginLogin(ctx context.Context, input BeginOIDCLoginInput) (string, error)
	Complete(ctx context.Context, input CompleteOIDCInput) (entity.OIDCExchange, error)
}

type SaveOIDCConnectionInput struct {
	WorkspaceID  uuid.UUID
	Issuer       string
	Endpoints    *entity.OIDCEndpoints
	ClientID     string
	ClientSecret string
	Scopes       []string
	GroupsClaim  string
	Provisioning bool
}

type BeginOIDCLoginInput struct {
	WorkspaceSlug string
	ReturnTo      string
}

type CompleteOIDCInput struct {
	State  string
	Code   string
	Client entity.SessionClient
}

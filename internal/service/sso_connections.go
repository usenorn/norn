package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=sso_connections.go -destination=ssoconnection/mock_sso_connections.go -package=ssoconnection -mock_names=SSOConnections=MockSSOConnections

type SSOConnections interface {
	Protocol(ctx context.Context, workspaceID uuid.UUID) (entity.SSOProtocol, error)
	SignIn(ctx context.Context, slug string) (entity.WorkspaceSignIn, error)
	Remove(ctx context.Context, workspaceID uuid.UUID) error

	Get(ctx context.Context, workspaceID uuid.UUID) (entity.OIDCConnection, error)
	Save(ctx context.Context, input SaveOIDCConnectionInput) (entity.OIDCConnection, error)
	Discover(ctx context.Context, workspaceID uuid.UUID, issuer string) (entity.OIDCEndpoints, error)
	BeginTest(ctx context.Context, workspaceID uuid.UUID) (string, error)
	BeginLink(ctx context.Context, workspaceID uuid.UUID) (string, error)
	BeginLogin(ctx context.Context, input BeginOIDCLoginInput) (string, error)
	Complete(ctx context.Context, input CompleteOIDCInput) (entity.SSOExchange, error)

	GetSAML(ctx context.Context, workspaceID uuid.UUID) (entity.SAMLConnection, error)
	SaveSAML(ctx context.Context, input SaveSAMLConnectionInput) (entity.SAMLConnection, error)
	ReadSAMLMetadata(ctx context.Context, input ReadSAMLMetadataInput) (entity.SAMLDescriptor, error)
	PublishSAMLMetadata(ctx context.Context, workspaceSlug string) ([]byte, error)
	BeginSAMLTest(ctx context.Context, workspaceID uuid.UUID) (string, error)
	BeginSAMLLogin(ctx context.Context, input BeginOIDCLoginInput) (string, error)
	CompleteSAML(ctx context.Context, input CompleteSAMLInput) (entity.SSOExchange, error)
	SweepCertificates(ctx context.Context) error
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

type SaveSAMLConnectionInput struct {
	WorkspaceID       uuid.UUID
	MetadataURL       string
	Metadata          string
	Descriptor        *entity.SAMLDescriptor
	AllowIDPInitiated bool
	Mapping           entity.SAMLAttributeMapping
	Provisioning      bool
}

type ReadSAMLMetadataInput struct {
	WorkspaceID uuid.UUID
	MetadataURL string
	Metadata    string
}

type CompleteSAMLInput struct {
	WorkspaceSlug string
	RelayState    string
	Response      []byte
	Client        entity.SessionClient
}

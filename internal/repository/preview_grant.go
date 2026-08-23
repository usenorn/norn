package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=preview_grant.go -destination=previewgrant/mock_preview_grant.go -package=previewgrant -mock_names=PreviewGrant=MockPreviewGrant

type PreviewGrant interface {
	Issue(ctx context.Context, grant entity.PreviewGrant, ttl time.Duration) (string, error)
	Read(ctx context.Context, token string) (entity.PreviewGrant, error)
	Revoke(ctx context.Context, token string) error
	IssueTicket(ctx context.Context, grant entity.PreviewGrant, ttl time.Duration) (string, error)
	RedeemTicket(ctx context.Context, ticket string) (entity.PreviewGrant, error)
	RevokeLink(ctx context.Context, linkID uuid.UUID) error
	FirstLook(ctx context.Context, viewer string, window time.Duration) (bool, error)
	Attempt(ctx context.Context, subject string, window time.Duration) (int, error)
	GrantGateway(
		ctx context.Context,
		tokenHash []byte,
		gateway entity.PreviewGateway,
		ttl time.Duration,
	) error
	ResolveGateway(ctx context.Context, tokenHash []byte) (entity.PreviewGateway, error)
	RevokeGateway(ctx context.Context, gatewayID uuid.UUID) error
}

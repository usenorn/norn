package repository

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=blob_grant.go -destination=blobgrant/mock_blob_grant.go -package=blobgrant -mock_names=BlobGrant=MockBlobGrant

type BlobGrant interface {
	Issue(ctx context.Context, grant entity.BlobGrant, ttl time.Duration) (string, error)
	Read(ctx context.Context, token string) (entity.BlobGrant, error)
	Revoke(ctx context.Context, token string) error
}

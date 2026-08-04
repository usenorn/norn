package repository

import (
	"context"
	"io"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=blob.go -destination=blob/mock_blob.go -package=blob -mock_names=Blob=MockBlob

type Blob interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Open(ctx context.Context, key string) (io.ReadSeekCloser, error)
	Sniff(ctx context.Context, key string) (string, error)
	Stat(ctx context.Context, key string) (entity.BlobObject, error)
	Delete(ctx context.Context, key string) error
	RemoveAll(ctx context.Context, prefix string) error
	PresignPut(ctx context.Context, key string, ttl time.Duration) (entity.BlobTicket, error)
	PresignGet(ctx context.Context, key string, serve entity.ServeSpec, ttl time.Duration) (string, error)
}

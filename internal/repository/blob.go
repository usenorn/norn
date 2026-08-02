package repository

import (
	"context"
	"io"
)

//go:generate go tool mockgen -source=blob.go -destination=blob/mock_blob.go -package=blob -mock_names=Blob=MockBlob

type Blob interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	URL(key string) string
}

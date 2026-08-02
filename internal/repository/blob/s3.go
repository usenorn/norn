package blob

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/usenorn/norn/internal/pkg/objectstore"
	"github.com/usenorn/norn/internal/repository"
)

type blobRepository struct {
	client *objectstore.Client
}

func New(client *objectstore.Client) repository.Blob {
	return &blobRepository{client: client}
}

func (r *blobRepository) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(r.client.Bucket()),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("put object %q: %w", key, err)
	}

	return nil
}

func (r *blobRepository) Delete(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.client.Bucket()),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}

	return nil
}

func (r *blobRepository) URL(key string) string {
	if key == "" {
		return ""
	}

	return strings.TrimSuffix(r.client.PublicBaseURL(), "/") + "/" + key
}

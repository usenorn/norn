package blobgrant

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

const (
	grantKeyPrefix = "blob-grant:"
	grantTokenSize = 32
)

type storedGrant struct {
	Purpose     string    `json:"purpose"`
	Key         string    `json:"key"`
	MaxBytes    int64     `json:"max_bytes"`
	ContentType string    `json:"content_type"`
	Disposition string    `json:"disposition"`
	FileName    string    `json:"file_name"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type grantRepository struct {
	client *valkey.Client
}

func New(client *valkey.Client) repository.BlobGrant {
	return &grantRepository{client: client}
}

func key(token string) string {
	digest := sha256.Sum256([]byte(token))

	return grantKeyPrefix + hex.EncodeToString(digest[:])
}

func opaque() (string, error) {
	buffer := make([]byte, grantTokenSize)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate blob grant: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func (r *grantRepository) Issue(
	ctx context.Context,
	grant entity.BlobGrant,
	ttl time.Duration,
) (string, error) {
	token, err := opaque()
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(storedGrant{
		Purpose:     string(grant.Purpose),
		Key:         grant.Key,
		MaxBytes:    grant.MaxBytes,
		ContentType: grant.Serve.ContentType,
		Disposition: grant.Serve.Disposition,
		FileName:    grant.Serve.FileName,
		ExpiresAt:   grant.ExpiresAt,
	})
	if err != nil {
		return "", fmt.Errorf("encode blob grant: %w", err)
	}

	if err := r.client.Set(ctx, key(token), payload, ttl).Err(); err != nil {
		return "", fmt.Errorf("store blob grant: %w", err)
	}

	return token, nil
}

func (r *grantRepository) Read(ctx context.Context, token string) (entity.BlobGrant, error) {
	payload, err := r.client.Get(ctx, key(token)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.BlobGrant{}, entity.ErrBlobGrantNotFound
		}

		return entity.BlobGrant{}, fmt.Errorf("read blob grant: %w", err)
	}

	var stored storedGrant
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.BlobGrant{}, fmt.Errorf("decode blob grant: %w", err)
	}

	return entity.BlobGrant{
		Purpose:  entity.BlobGrantPurpose(stored.Purpose),
		Key:      stored.Key,
		MaxBytes: stored.MaxBytes,
		Serve: entity.ServeSpec{
			ContentType: stored.ContentType,
			Disposition: stored.Disposition,
			FileName:    stored.FileName,
		},
		ExpiresAt: stored.ExpiresAt,
	}, nil
}

func (r *grantRepository) Revoke(ctx context.Context, token string) error {
	if err := r.client.Del(ctx, key(token)).Err(); err != nil {
		return fmt.Errorf("revoke blob grant: %w", err)
	}

	return nil
}

package samlreplay

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

const replayKeyPrefix = "saml-assertion:"

const claimed = "1"

type replayRepository struct {
	client *valkey.Client
	ttl    time.Duration
}

func New(client *valkey.Client, cfg config.SAML) repository.SAMLReplay {
	return &replayRepository{client: client, ttl: cfg.ReplayTTL}
}

func key(workspaceID uuid.UUID, assertionID string) string {
	return replayKeyPrefix + workspaceID.String() + ":" + assertionID
}

func (r *replayRepository) Claim(
	ctx context.Context,
	workspaceID uuid.UUID,
	assertionID string,
) (bool, error) {
	err := r.client.SetArgs(ctx, key(workspaceID, assertionID), claimed, redis.SetArgs{
		Mode: "NX",
		TTL:  r.ttl,
	}).Err()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}

		return false, fmt.Errorf("claim saml assertion: %w", err)
	}

	return true, nil
}

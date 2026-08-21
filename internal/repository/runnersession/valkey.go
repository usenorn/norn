package runnersession

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

const (
	noncePrefix  = "runner-nonce:"
	accessPrefix = "runner-access:"
	ticketPrefix = "runner-ticket:"

	claimed = "1"
)

type sessionRepository struct {
	client   *valkey.Client
	nonceTTL time.Duration
}

func New(client *valkey.Client, cfg config.Runner) repository.RunnerSession {
	return &sessionRepository{client: client, nonceTTL: cfg.NonceTTL}
}

func nonceKey(runnerID uuid.UUID, nonce string) string {
	return noncePrefix + runnerID.String() + ":" + nonce
}

func secretKey(prefix string, hash []byte) string {
	return prefix + hex.EncodeToString(hash)
}

func (r *sessionRepository) ClaimNonce(
	ctx context.Context,
	runnerID uuid.UUID,
	nonce string,
) (bool, error) {
	err := r.client.SetArgs(ctx, nonceKey(runnerID, nonce), claimed, redis.SetArgs{
		Mode: "NX",
		TTL:  r.nonceTTL,
	}).Err()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}

		return false, fmt.Errorf("claim runner nonce: %w", err)
	}

	return true, nil
}

func (r *sessionRepository) Grant(
	ctx context.Context,
	tokenHash []byte,
	runnerID uuid.UUID,
	ttl time.Duration,
) error {
	if err := r.client.Set(ctx, secretKey(accessPrefix, tokenHash), runnerID.String(), ttl).Err(); err != nil {
		return fmt.Errorf("grant runner access: %w", err)
	}

	return nil
}

func (r *sessionRepository) Resolve(ctx context.Context, tokenHash []byte) (uuid.UUID, error) {
	return r.read(ctx, secretKey(accessPrefix, tokenHash), false)
}

func (r *sessionRepository) IssueTicket(
	ctx context.Context,
	ticketHash []byte,
	runnerID uuid.UUID,
	ttl time.Duration,
) error {
	if err := r.client.Set(ctx, secretKey(ticketPrefix, ticketHash), runnerID.String(), ttl).Err(); err != nil {
		return fmt.Errorf("issue runner ticket: %w", err)
	}

	return nil
}

func (r *sessionRepository) RedeemTicket(ctx context.Context, ticketHash []byte) (uuid.UUID, error) {
	return r.read(ctx, secretKey(ticketPrefix, ticketHash), true)
}

func (r *sessionRepository) read(ctx context.Context, key string, spend bool) (uuid.UUID, error) {
	var (
		value string
		err   error
	)

	if spend {
		value, err = r.client.GetDel(ctx, key).Result()
	} else {
		value, err = r.client.Get(ctx, key).Result()
	}

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return uuid.Nil, entity.ErrRunnerCredentialInvalid
		}

		return uuid.Nil, fmt.Errorf("read runner session: %w", err)
	}

	runnerID, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse runner session id: %w", err)
	}

	return runnerID, nil
}

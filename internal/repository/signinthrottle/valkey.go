package signinthrottle

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

const (
	failuresKeyPrefix = "signin-failures:"
	lockKeyPrefix     = "signin-lock:"
	addressKeyPrefix  = "signin-address:"

	lockValue = "1"
)

type signInThrottleRepository struct {
	client *valkey.Client
}

func New(client *valkey.Client) repository.SignInThrottle {
	return &signInThrottleRepository{client: client}
}

func (r *signInThrottleRepository) failuresKey(subjectHash string) string {
	return failuresKeyPrefix + subjectHash
}

func (r *signInThrottleRepository) lockKey(subjectHash string) string {
	return lockKeyPrefix + subjectHash
}

func (r *signInThrottleRepository) addressKey(ip netip.Addr) string {
	return addressKeyPrefix + ip.String()
}

func (r *signInThrottleRepository) lockedUntil(ctx context.Context, subjectHash string) (time.Time, error) {
	ttl, err := r.client.PTTL(ctx, r.lockKey(subjectHash)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return time.Time{}, nil
		}

		return time.Time{}, fmt.Errorf("read sign-in lock: %w", err)
	}

	if ttl <= 0 {
		return time.Time{}, nil
	}

	return time.Now().UTC().Add(ttl), nil
}

func (r *signInThrottleRepository) Get(ctx context.Context, subjectHash string) (entity.SignInThrottle, error) {
	failures, err := r.client.Get(ctx, r.failuresKey(subjectHash)).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return entity.SignInThrottle{}, fmt.Errorf("read sign-in failures: %w", err)
	}

	lockedUntil, err := r.lockedUntil(ctx, subjectHash)
	if err != nil {
		return entity.SignInThrottle{}, err
	}

	return entity.SignInThrottle{Failures: failures, LockedUntil: lockedUntil}, nil
}

func (r *signInThrottleRepository) RecordFailure(ctx context.Context, subjectHash string) (entity.SignInThrottle, error) {
	key := r.failuresKey(subjectHash)

	failures, err := r.count(ctx, key, entity.SignInFailureWindow)
	if err != nil {
		return entity.SignInThrottle{}, fmt.Errorf("increment sign-in failures: %w", err)
	}

	if failures < entity.SignInMaxFailures {
		return entity.SignInThrottle{Failures: int(failures)}, nil
	}

	if err := r.client.SetArgs(ctx, r.lockKey(subjectHash), lockValue, redis.SetArgs{
		Mode: "NX",
		TTL:  entity.SignInLockDuration,
	}).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return entity.SignInThrottle{}, fmt.Errorf("set sign-in lock: %w", err)
	}

	if err := r.client.Del(ctx, key).Err(); err != nil {
		return entity.SignInThrottle{}, fmt.Errorf("reset sign-in failures: %w", err)
	}

	lockedUntil, err := r.lockedUntil(ctx, subjectHash)
	if err != nil {
		return entity.SignInThrottle{}, err
	}

	return entity.SignInThrottle{Failures: int(failures), LockedUntil: lockedUntil}, nil
}

func (r *signInThrottleRepository) Clear(ctx context.Context, subjectHash string) error {
	if err := r.client.Del(ctx, r.failuresKey(subjectHash), r.lockKey(subjectHash)).Err(); err != nil {
		return fmt.Errorf("clear sign-in throttle: %w", err)
	}

	return nil
}

func (r *signInThrottleRepository) RecordAddressAttempt(ctx context.Context, ip netip.Addr) (int, error) {
	key := r.addressKey(ip)

	attempts, err := r.count(ctx, key, entity.SignInAddressWindow)
	if err != nil {
		return 0, fmt.Errorf("increment sign-in address attempts: %w", err)
	}

	return int(attempts), nil
}

func (r *signInThrottleRepository) count(
	ctx context.Context,
	key string,
	window time.Duration,
) (int64, error) {
	err := r.client.SetArgs(ctx, key, 0, redis.SetArgs{Mode: "NX", TTL: window}).Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}

	return r.client.Incr(ctx, key).Result()
}

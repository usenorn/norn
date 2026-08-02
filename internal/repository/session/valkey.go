package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

const (
	indexKeyPrefix   = "account-sessions:"
	revokedKeyPrefix = "account-revoked-at:"
)

type storedSession struct {
	ID                uuid.UUID  `json:"id"`
	AccountID         uuid.UUID  `json:"account_id"`
	AuthMethod        string     `json:"auth_method"`
	UserAgent         string     `json:"user_agent"`
	IP                netip.Addr `json:"ip"`
	CountryCode       string     `json:"country_code"`
	City              string     `json:"city"`
	IssuedAt          time.Time  `json:"issued_at"`
	LastUsedAt        time.Time  `json:"last_used_at"`
	IdleExpiresAt     time.Time  `json:"idle_expires_at"`
	AbsoluteExpiresAt time.Time  `json:"absolute_expires_at"`
}

func toStored(session entity.Session) storedSession {
	return storedSession{
		ID:                session.ID,
		AccountID:         session.AccountID,
		AuthMethod:        string(session.AuthMethod),
		UserAgent:         session.Client.UserAgent,
		IP:                session.Client.IP,
		CountryCode:       session.Client.Location.CountryCode,
		City:              session.Client.Location.City,
		IssuedAt:          session.IssuedAt,
		LastUsedAt:        session.LastUsedAt,
		IdleExpiresAt:     session.IdleExpiresAt,
		AbsoluteExpiresAt: session.AbsoluteExpiresAt,
	}
}

func toEntity(tokenHash string, stored storedSession) entity.Session {
	return entity.Session{
		ID:         stored.ID,
		TokenHash:  tokenHash,
		AccountID:  stored.AccountID,
		AuthMethod: entity.SessionAuthMethod(stored.AuthMethod),
		Client: entity.SessionClient{
			UserAgent: stored.UserAgent,
			IP:        stored.IP,
			Location:  entity.Location{CountryCode: stored.CountryCode, City: stored.City},
		},
		IssuedAt:          stored.IssuedAt,
		LastUsedAt:        stored.LastUsedAt,
		IdleExpiresAt:     stored.IdleExpiresAt,
		AbsoluteExpiresAt: stored.AbsoluteExpiresAt,
	}
}

type sessionRepository struct {
	client           *valkey.Client
	keyPrefix        string
	absoluteLifetime time.Duration
	maxPerAccount    int
}

func New(client *valkey.Client, cfg config.Session) repository.Session {
	return &sessionRepository{
		client:           client,
		keyPrefix:        cfg.KeyPrefix,
		absoluteLifetime: cfg.AbsoluteLifetime,
		maxPerAccount:    cfg.MaxPerAccount,
	}
}

func (r *sessionRepository) key(tokenHash string) string {
	return r.keyPrefix + tokenHash
}

func (r *sessionRepository) indexKey(accountID uuid.UUID) string {
	return indexKeyPrefix + accountID.String()
}

func (r *sessionRepository) revokedKey(accountID uuid.UUID) string {
	return revokedKeyPrefix + accountID.String()
}

func (r *sessionRepository) Create(ctx context.Context, session entity.Session) error {
	payload, err := json.Marshal(toStored(session))
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}

	ttl := time.Until(session.ExpiresAt())
	if ttl <= 0 {
		return fmt.Errorf("session expiry %s is not in the future", session.ExpiresAt())
	}

	indexKey := r.indexKey(session.AccountID)

	if err := r.client.SAdd(ctx, indexKey, session.TokenHash).Err(); err != nil {
		return fmt.Errorf("index session: %w", err)
	}

	if err := r.client.Expire(ctx, indexKey, r.absoluteLifetime).Err(); err != nil {
		return fmt.Errorf("bound session index lifetime: %w", err)
	}

	stored, err := r.client.SetNX(ctx, r.key(session.TokenHash), payload, ttl).Result()
	if err != nil {
		return fmt.Errorf("store session: %w", err)
	}

	if !stored {
		return fmt.Errorf("session token collision for account %s", session.AccountID)
	}

	return r.enforceMaxPerAccount(ctx, session.AccountID, session.TokenHash)
}

func (r *sessionRepository) enforceMaxPerAccount(ctx context.Context, accountID uuid.UUID, keepTokenHash string) error {
	sessions, err := r.ListByAccountID(ctx, accountID)
	if err != nil {
		return err
	}

	if len(sessions) <= r.maxPerAccount {
		return nil
	}

	slices.SortFunc(sessions, func(a, b entity.Session) int {
		return a.IssuedAt.Compare(b.IssuedAt)
	})

	for _, evicted := range sessions[:len(sessions)-r.maxPerAccount] {
		if evicted.TokenHash == keepTokenHash {
			continue
		}

		if err := r.Delete(ctx, evicted.TokenHash); err != nil {
			return err
		}
	}

	return nil
}

func (r *sessionRepository) Get(ctx context.Context, tokenHash string) (entity.Session, error) {
	payload, err := r.client.Get(ctx, r.key(tokenHash)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.Session{}, entity.ErrSessionNotFound
		}

		return entity.Session{}, fmt.Errorf("load session: %w", err)
	}

	var stored storedSession
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.Session{}, fmt.Errorf("decode session: %w", err)
	}

	return toEntity(tokenHash, stored), nil
}

func (r *sessionRepository) Touch(ctx context.Context, session entity.Session) error {
	payload, err := json.Marshal(toStored(session))
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}

	ttl := time.Until(session.ExpiresAt())
	if ttl <= 0 {
		return entity.ErrSessionNotFound
	}

	result, err := r.client.SetArgs(ctx, r.key(session.TokenHash), payload, redis.SetArgs{
		Mode: "XX",
		TTL:  ttl,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.ErrSessionNotFound
		}

		return fmt.Errorf("refresh session: %w", err)
	}

	if result == "" {
		return entity.ErrSessionNotFound
	}

	return nil
}

func (r *sessionRepository) ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]entity.Session, error) {
	indexKey := r.indexKey(accountID)

	tokenHashes, err := r.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("list session index: %w", err)
	}

	if len(tokenHashes) == 0 {
		return nil, nil
	}

	pipeline := r.client.Pipeline()

	commands := make([]*redis.StringCmd, len(tokenHashes))
	for i, tokenHash := range tokenHashes {
		commands[i] = pipeline.Get(ctx, r.key(tokenHash))
	}

	if _, err := pipeline.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("load indexed sessions: %w", err)
	}

	var (
		sessions []entity.Session
		stale    []any
	)

	for i, command := range commands {
		payload, err := command.Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				stale = append(stale, tokenHashes[i])

				continue
			}

			return nil, fmt.Errorf("load indexed session: %w", err)
		}

		var stored storedSession
		if err := json.Unmarshal(payload, &stored); err != nil {
			return nil, fmt.Errorf("decode session: %w", err)
		}

		sessions = append(sessions, toEntity(tokenHashes[i], stored))
	}

	if len(stale) > 0 {
		if err := r.client.SRem(ctx, indexKey, stale...).Err(); err != nil {
			return nil, fmt.Errorf("prune session index: %w", err)
		}
	}

	return sessions, nil
}

func (r *sessionRepository) Delete(ctx context.Context, tokenHash string) error {
	session, err := r.Get(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, entity.ErrSessionNotFound) {
			return nil
		}

		return err
	}

	if err := r.client.Del(ctx, r.key(tokenHash)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	if err := r.client.SRem(ctx, r.indexKey(session.AccountID), tokenHash).Err(); err != nil {
		return fmt.Errorf("unindex session: %w", err)
	}

	return nil
}

func (r *sessionRepository) DeleteByID(ctx context.Context, accountID, sessionID uuid.UUID) error {
	sessions, err := r.ListByAccountID(ctx, accountID)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		if session.ID == sessionID {
			return r.Delete(ctx, session.TokenHash)
		}
	}

	return entity.ErrSessionNotFound
}

func (r *sessionRepository) DeleteByAccountID(ctx context.Context, accountID uuid.UUID) error {
	return r.DeleteByAccountIDExcept(ctx, accountID, "")
}

func (r *sessionRepository) DeleteByAccountIDExcept(ctx context.Context, accountID uuid.UUID, keepTokenHash string) error {
	indexKey := r.indexKey(accountID)

	tokenHashes, err := r.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		return fmt.Errorf("list session index: %w", err)
	}

	var (
		keys    []string
		members []any
	)

	for _, tokenHash := range tokenHashes {
		if tokenHash == keepTokenHash {
			continue
		}

		keys = append(keys, r.key(tokenHash))
		members = append(members, tokenHash)
	}

	if len(keys) == 0 {
		return nil
	}

	if err := r.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("delete account sessions: %w", err)
	}

	if err := r.client.SRem(ctx, indexKey, members...).Err(); err != nil {
		return fmt.Errorf("prune session index: %w", err)
	}

	return nil
}

func (r *sessionRepository) RevokedAt(ctx context.Context, accountID uuid.UUID) (time.Time, error) {
	raw, err := r.client.Get(ctx, r.revokedKey(accountID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return time.Time{}, nil
		}

		return time.Time{}, fmt.Errorf("load account revocation: %w", err)
	}

	millis, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("decode account revocation: %w", err)
	}

	return time.UnixMilli(millis).UTC(), nil
}

func (r *sessionRepository) MarkRevoked(ctx context.Context, accountID uuid.UUID, at time.Time) error {
	value := strconv.FormatInt(at.UnixMilli(), 10)

	if err := r.client.Set(ctx, r.revokedKey(accountID), value, r.absoluteLifetime).Err(); err != nil {
		return fmt.Errorf("mark account sessions revoked: %w", err)
	}

	return nil
}

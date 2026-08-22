package signinchallenge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

const challengeKeyPrefix = "signin-challenge:"

type storedChallenge struct {
	AccountID   uuid.UUID `json:"account_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CodeHash    []byte    `json:"code_hash"`
	Attempts    int       `json:"attempts"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	UserAgent   string    `json:"user_agent"`
	IP          string    `json:"ip"`
	CountryCode string    `json:"country_code"`
	City        string    `json:"city"`
}

type challengeRepository struct {
	client *valkey.Client
}

func New(client *valkey.Client) repository.SignInChallenge {
	return &challengeRepository{client: client}
}

func key(id string) string { return challengeKeyPrefix + id }

func (r *challengeRepository) Put(
	ctx context.Context,
	id string,
	challenge entity.SignInChallenge,
) error {
	stored := storedChallenge{
		AccountID:   challenge.AccountID,
		Email:       challenge.Email,
		DisplayName: challenge.DisplayName,
		CodeHash:    challenge.CodeHash,
		Attempts:    challenge.Attempts,
		IssuedAt:    challenge.IssuedAt,
		ExpiresAt:   challenge.ExpiresAt,
		UserAgent:   challenge.Client.UserAgent,
		CountryCode: challenge.Client.Location.CountryCode,
		City:        challenge.Client.Location.City,
	}

	if challenge.Client.IP.IsValid() {
		stored.IP = challenge.Client.IP.String()
	}

	payload, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encode sign-in challenge: %w", err)
	}

	ttl := time.Until(challenge.ExpiresAt)
	if ttl <= 0 {
		return nil
	}

	if err := r.client.Set(ctx, key(id), payload, ttl).Err(); err != nil {
		return fmt.Errorf("store sign-in challenge: %w", err)
	}

	return nil
}

func (r *challengeRepository) Get(ctx context.Context, id string) (entity.SignInChallenge, error) {
	payload, err := r.client.Get(ctx, key(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.SignInChallenge{}, entity.ErrSignInChallengeNotFound
		}

		return entity.SignInChallenge{}, fmt.Errorf("read sign-in challenge: %w", err)
	}

	var stored storedChallenge
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.SignInChallenge{}, fmt.Errorf("decode sign-in challenge: %w", err)
	}

	challenge := entity.SignInChallenge{
		AccountID:   stored.AccountID,
		Email:       stored.Email,
		DisplayName: stored.DisplayName,
		CodeHash:    stored.CodeHash,
		Attempts:    stored.Attempts,
		IssuedAt:    stored.IssuedAt,
		ExpiresAt:   stored.ExpiresAt,
		Client: entity.SessionClient{
			UserAgent: stored.UserAgent,
			Location:  entity.Location{CountryCode: stored.CountryCode, City: stored.City},
		},
	}

	if stored.IP != "" {
		address, err := netip.ParseAddr(stored.IP)
		if err != nil {
			return entity.SignInChallenge{}, fmt.Errorf("decode sign-in challenge address: %w", err)
		}

		challenge.Client.IP = address
	}

	return challenge, nil
}

func (r *challengeRepository) Delete(ctx context.Context, id string) error {
	if err := r.client.Del(ctx, key(id)).Err(); err != nil {
		return fmt.Errorf("discard sign-in challenge: %w", err)
	}

	return nil
}

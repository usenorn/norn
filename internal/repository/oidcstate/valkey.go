package oidcstate

import (
	"context"
	"encoding/json"
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

const stateKeyPrefix = "oidc-state:"

type storedState struct {
	Purpose     string    `json:"purpose"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	AccountID   uuid.UUID `json:"account_id"`
	Nonce       string    `json:"nonce"`
	Verifier    string    `json:"verifier"`
	ReturnTo    string    `json:"return_to"`
	CreatedAt   time.Time `json:"created_at"`
}

type stateRepository struct {
	client *valkey.Client
	ttl    time.Duration
}

func New(client *valkey.Client, cfg config.OIDC) repository.OIDCState {
	return &stateRepository{client: client, ttl: cfg.StateTTL}
}

func key(state string) string { return stateKeyPrefix + state }

func (r *stateRepository) Put(ctx context.Context, state string, attempt entity.OIDCState) error {
	payload, err := json.Marshal(storedState{
		Purpose:     string(attempt.Purpose),
		WorkspaceID: attempt.WorkspaceID,
		AccountID:   attempt.AccountID,
		Nonce:       attempt.Nonce,
		Verifier:    attempt.Verifier,
		ReturnTo:    attempt.ReturnTo,
		CreatedAt:   attempt.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("encode oidc state: %w", err)
	}

	if err := r.client.Set(ctx, key(state), payload, r.ttl).Err(); err != nil {
		return fmt.Errorf("store oidc state: %w", err)
	}

	return nil
}

func (r *stateRepository) Take(ctx context.Context, state string) (entity.OIDCState, error) {
	payload, err := r.client.GetDel(ctx, key(state)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.OIDCState{}, entity.ErrSSOStateNotFound
		}

		return entity.OIDCState{}, fmt.Errorf("take oidc state: %w", err)
	}

	var stored storedState
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.OIDCState{}, fmt.Errorf("decode oidc state: %w", err)
	}

	return entity.OIDCState{
		Purpose:     entity.SSOPurpose(stored.Purpose),
		WorkspaceID: stored.WorkspaceID,
		AccountID:   stored.AccountID,
		Nonce:       stored.Nonce,
		Verifier:    stored.Verifier,
		ReturnTo:    stored.ReturnTo,
		CreatedAt:   stored.CreatedAt,
	}, nil
}

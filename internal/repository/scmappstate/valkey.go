package scmappstate

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

const stateKeyPrefix = "scm-app-state:"

type storedState struct {
	Purpose       string                   `json:"purpose"`
	Provider      string                   `json:"provider"`
	WorkspaceID   uuid.UUID                `json:"workspace_id"`
	AccountID     uuid.UUID                `json:"account_id"`
	Organization  string                   `json:"organization"`
	Installations []entity.SCMInstallation `json:"installations"`
	CreatedAt     time.Time                `json:"created_at"`
}

type stateRepository struct {
	client *valkey.Client
	ttl    time.Duration
}

func New(client *valkey.Client, cfg config.SourceControl) repository.SCMAppState {
	return &stateRepository{client: client, ttl: cfg.AppStateTTL}
}

func key(state string) string { return stateKeyPrefix + state }

func (r *stateRepository) Put(ctx context.Context, state string, attempt entity.SCMAppState) error {
	payload, err := json.Marshal(storedState{
		Purpose:       string(attempt.Purpose),
		Provider:      string(attempt.Provider),
		WorkspaceID:   attempt.WorkspaceID,
		AccountID:     attempt.AccountID,
		Organization:  attempt.Organization,
		Installations: attempt.Installations,
		CreatedAt:     attempt.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("encode source control application state: %w", err)
	}

	if err := r.client.Set(ctx, key(state), payload, r.ttl).Err(); err != nil {
		return fmt.Errorf("store source control application state: %w", err)
	}

	return nil
}

func (r *stateRepository) Take(ctx context.Context, state string) (entity.SCMAppState, error) {
	if state == "" {
		return entity.SCMAppState{}, entity.ErrSCMAppStateNotFound
	}

	payload, err := r.client.GetDel(ctx, key(state)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.SCMAppState{}, entity.ErrSCMAppStateNotFound
		}

		return entity.SCMAppState{}, fmt.Errorf("take source control application state: %w", err)
	}

	var stored storedState
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.SCMAppState{}, fmt.Errorf("decode source control application state: %w", err)
	}

	return entity.SCMAppState{
		Purpose:       entity.SCMAppPurpose(stored.Purpose),
		Provider:      entity.SCMProvider(stored.Provider),
		WorkspaceID:   stored.WorkspaceID,
		AccountID:     stored.AccountID,
		Organization:  stored.Organization,
		Installations: stored.Installations,
		CreatedAt:     stored.CreatedAt,
	}, nil
}

package samlrequest

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

const requestKeyPrefix = "saml-request:"

type storedAttempt struct {
	Purpose     string    `json:"purpose"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	RequestID   string    `json:"request_id"`
	Correlator  string    `json:"correlator"`
	ReturnTo    string    `json:"return_to"`
	CreatedAt   time.Time `json:"created_at"`
}

type requestRepository struct {
	client *valkey.Client
	ttl    time.Duration
}

func New(client *valkey.Client, cfg config.SAML) repository.SAMLRequest {
	return &requestRepository{client: client, ttl: cfg.StateTTL}
}

func key(relayState string) string { return requestKeyPrefix + relayState }

func (r *requestRepository) Put(
	ctx context.Context,
	relayState string,
	attempt entity.SAMLAttempt,
) error {
	payload, err := json.Marshal(storedAttempt{
		Purpose:     string(attempt.Purpose),
		WorkspaceID: attempt.WorkspaceID,
		RequestID:   attempt.RequestID,
		Correlator:  attempt.Correlator,
		ReturnTo:    attempt.ReturnTo,
		CreatedAt:   attempt.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("encode saml attempt: %w", err)
	}

	if err := r.client.Set(ctx, key(relayState), payload, r.ttl).Err(); err != nil {
		return fmt.Errorf("store saml attempt: %w", err)
	}

	return nil
}

func (r *requestRepository) Take(
	ctx context.Context,
	relayState string,
) (entity.SAMLAttempt, error) {
	payload, err := r.client.GetDel(ctx, key(relayState)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.SAMLAttempt{}, entity.ErrSSOStateNotFound
		}

		return entity.SAMLAttempt{}, fmt.Errorf("take saml attempt: %w", err)
	}

	var stored storedAttempt
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.SAMLAttempt{}, fmt.Errorf("decode saml attempt: %w", err)
	}

	return entity.SAMLAttempt{
		Purpose:     entity.SSOPurpose(stored.Purpose),
		WorkspaceID: stored.WorkspaceID,
		RequestID:   stored.RequestID,
		Correlator:  stored.Correlator,
		ReturnTo:    stored.ReturnTo,
		CreatedAt:   stored.CreatedAt,
	}, nil
}

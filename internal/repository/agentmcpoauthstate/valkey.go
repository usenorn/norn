package agentmcpoauthstate

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

const stateKeyPrefix = "agent-mcp-oauth-state:"

type storedState struct {
	WorkspaceID   uuid.UUID `json:"workspace_id"`
	ServerID      uuid.UUID `json:"server_id"`
	AccountID     uuid.UUID `json:"account_id"`
	Verifier      string    `json:"verifier"`
	Issuer        string    `json:"issuer"`
	TokenEndpoint string    `json:"token_endpoint"`
	Resource      string    `json:"resource"`
	Scopes        []string  `json:"scopes"`
	ClientID      string    `json:"client_id"`
	ClientSecret  string    `json:"client_secret"`
	ReturnTo      string    `json:"return_to"`
	CreatedAt     time.Time `json:"created_at"`
}

type stateRepository struct {
	client *valkey.Client
	ttl    time.Duration
}

func New(client *valkey.Client, cfg config.AgentTooling) repository.AgentMCPOAuthState {
	return &stateRepository{client: client, ttl: cfg.OAuthStateTTL}
}

func key(state string) string { return stateKeyPrefix + state }

func (r *stateRepository) Put(ctx context.Context, state string, attempt entity.AgentMCPOAuthState) error {
	payload, err := json.Marshal(storedState{
		WorkspaceID:   attempt.WorkspaceID,
		ServerID:      attempt.ServerID,
		AccountID:     attempt.AccountID,
		Verifier:      attempt.Verifier,
		Issuer:        attempt.Issuer,
		TokenEndpoint: attempt.TokenEndpoint,
		Resource:      attempt.Resource,
		Scopes:        attempt.Scopes,
		ClientID:      attempt.Client.ID,
		ClientSecret:  attempt.Client.Secret,
		ReturnTo:      attempt.ReturnTo,
		CreatedAt:     attempt.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("encode mcp oauth state: %w", err)
	}

	if err := r.client.Set(ctx, key(state), payload, r.ttl).Err(); err != nil {
		return fmt.Errorf("store mcp oauth state: %w", err)
	}

	return nil
}

func (r *stateRepository) Take(ctx context.Context, state string) (entity.AgentMCPOAuthState, error) {
	if state == "" {
		return entity.AgentMCPOAuthState{}, entity.ErrAgentMCPOAuthStateNotFound
	}

	payload, err := r.client.GetDel(ctx, key(state)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.AgentMCPOAuthState{}, entity.ErrAgentMCPOAuthStateNotFound
		}

		return entity.AgentMCPOAuthState{}, fmt.Errorf("take mcp oauth state: %w", err)
	}

	var stored storedState
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.AgentMCPOAuthState{}, fmt.Errorf("decode mcp oauth state: %w", err)
	}

	return entity.AgentMCPOAuthState{
		WorkspaceID:   stored.WorkspaceID,
		ServerID:      stored.ServerID,
		AccountID:     stored.AccountID,
		Verifier:      stored.Verifier,
		Issuer:        stored.Issuer,
		TokenEndpoint: stored.TokenEndpoint,
		Resource:      stored.Resource,
		Scopes:        stored.Scopes,
		Client:        entity.AgentMCPClient{ID: stored.ClientID, Secret: stored.ClientSecret},
		ReturnTo:      stored.ReturnTo,
		CreatedAt:     stored.CreatedAt,
	}, nil
}

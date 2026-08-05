package mcpauthstate

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

const (
	requestKeyPrefix = "mcp-authreq:"
	codeKeyPrefix    = "mcp-code:"
)

type storedRequest struct {
	ClientID      uuid.UUID `json:"client_id"`
	ClientName    string    `json:"client_name"`
	RedirectURI   string    `json:"redirect_uri"`
	Capability    string    `json:"capability"`
	State         string    `json:"state"`
	CodeChallenge string    `json:"code_challenge"`
	Resource      string    `json:"resource"`
	CreatedAt     time.Time `json:"created_at"`
}

type storedCode struct {
	ClientID      uuid.UUID `json:"client_id"`
	AccountID     uuid.UUID `json:"account_id"`
	RedirectURI   string    `json:"redirect_uri"`
	Capability    string    `json:"capability"`
	CodeChallenge string    `json:"code_challenge"`
}

type authStateRepository struct {
	client     *valkey.Client
	requestTTL time.Duration
	codeTTL    time.Duration
}

func New(client *valkey.Client, cfg config.MCP) repository.MCPAuthState {
	return &authStateRepository{
		client:     client,
		requestTTL: cfg.AuthRequestTTL,
		codeTTL:    cfg.AuthCodeTTL,
	}
}

func (r *authStateRepository) PutRequest(
	ctx context.Context,
	requestID string,
	request entity.MCPAuthRequest,
) error {
	payload, err := json.Marshal(storedRequest{
		ClientID:      request.ClientID,
		ClientName:    request.ClientName,
		RedirectURI:   request.RedirectURI,
		Capability:    string(request.Capability),
		State:         request.State,
		CodeChallenge: request.CodeChallenge,
		Resource:      request.Resource,
		CreatedAt:     request.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("encode mcp authorization request: %w", err)
	}

	if err := r.client.Set(ctx, requestKeyPrefix+requestID, payload, r.requestTTL).Err(); err != nil {
		return fmt.Errorf("store mcp authorization request: %w", err)
	}

	return nil
}

func (r *authStateRepository) GetRequest(
	ctx context.Context,
	requestID string,
) (entity.MCPAuthRequest, error) {
	payload, err := r.client.Get(ctx, requestKeyPrefix+requestID).Bytes()

	return decodeRequest(payload, err)
}

func (r *authStateRepository) TakeRequest(
	ctx context.Context,
	requestID string,
) (entity.MCPAuthRequest, error) {
	payload, err := r.client.GetDel(ctx, requestKeyPrefix+requestID).Bytes()

	return decodeRequest(payload, err)
}

func (r *authStateRepository) PutCode(ctx context.Context, code string, grant entity.MCPAuthCode) error {
	payload, err := json.Marshal(storedCode{
		ClientID:      grant.ClientID,
		AccountID:     grant.AccountID,
		RedirectURI:   grant.RedirectURI,
		Capability:    string(grant.Capability),
		CodeChallenge: grant.CodeChallenge,
	})
	if err != nil {
		return fmt.Errorf("encode mcp authorization code: %w", err)
	}

	if err := r.client.Set(ctx, codeKeyPrefix+code, payload, r.codeTTL).Err(); err != nil {
		return fmt.Errorf("store mcp authorization code: %w", err)
	}

	return nil
}

func (r *authStateRepository) TakeCode(ctx context.Context, code string) (entity.MCPAuthCode, error) {
	payload, err := r.client.GetDel(ctx, codeKeyPrefix+code).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.MCPAuthCode{}, entity.ErrMCPCodeInvalid
		}

		return entity.MCPAuthCode{}, fmt.Errorf("take mcp authorization code: %w", err)
	}

	var stored storedCode
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.MCPAuthCode{}, fmt.Errorf("decode mcp authorization code: %w", err)
	}

	return entity.MCPAuthCode{
		ClientID:      stored.ClientID,
		AccountID:     stored.AccountID,
		RedirectURI:   stored.RedirectURI,
		Capability:    entity.MCPCapability(stored.Capability),
		CodeChallenge: stored.CodeChallenge,
	}, nil
}

func decodeRequest(payload []byte, err error) (entity.MCPAuthRequest, error) {
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.MCPAuthRequest{}, entity.ErrMCPAuthRequestNotFound
		}

		return entity.MCPAuthRequest{}, fmt.Errorf("read mcp authorization request: %w", err)
	}

	var stored storedRequest
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.MCPAuthRequest{}, fmt.Errorf("decode mcp authorization request: %w", err)
	}

	return entity.MCPAuthRequest{
		ClientID:      stored.ClientID,
		ClientName:    stored.ClientName,
		RedirectURI:   stored.RedirectURI,
		Capability:    entity.MCPCapability(stored.Capability),
		State:         stored.State,
		CodeChallenge: stored.CodeChallenge,
		Resource:      stored.Resource,
		CreatedAt:     stored.CreatedAt,
	}, nil
}

package previewgrant

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

const (
	grantPrefix   = "preview-grant:"
	ticketPrefix  = "preview-ticket:"
	linkedPrefix  = "preview-link-grants:"
	gatewayPrefix = "preview-gateway:"
	holdingPrefix = "preview-gateway-grants:"
	lookPrefix    = "preview-look:"
	attemptPrefix = "preview-attempt:"

	grantTokenSize = 32
	looked         = "1"
)

type storedGrant struct {
	Audience    string    `json:"audience"`
	ExecutionID string    `json:"execution_id"`
	WorkspaceID string    `json:"workspace_id"`
	PreviewID   string    `json:"preview_id"`
	Path        string    `json:"path,omitempty"`
	AccountID   string    `json:"account_id"`
	LinkID      string    `json:"link_id"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type storedGateway struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type grantRepository struct {
	client *valkey.Client
}

func New(client *valkey.Client) repository.PreviewGrant {
	return &grantRepository{client: client}
}

func grantKey(token string) string {
	digest := sha256.Sum256([]byte(token))

	return grantPrefix + hex.EncodeToString(digest[:])
}

func ticketKey(ticket string) string {
	digest := sha256.Sum256([]byte(ticket))

	return ticketPrefix + hex.EncodeToString(digest[:])
}

func linkedKey(linkID uuid.UUID) string {
	return linkedPrefix + linkID.String()
}

func gatewayKey(tokenHash []byte) string {
	return gatewayPrefix + hex.EncodeToString(tokenHash)
}

func holdingKey(gatewayID uuid.UUID) string {
	return holdingPrefix + gatewayID.String()
}

func attemptKey(subject string) string {
	digest := sha256.Sum256([]byte(subject))

	return attemptPrefix + hex.EncodeToString(digest[:])
}

func lookKey(viewer string) string {
	digest := sha256.Sum256([]byte(viewer))

	return lookPrefix + hex.EncodeToString(digest[:])
}

func opaque() (string, error) {
	buffer := make([]byte, grantTokenSize)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate preview grant: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func (r *grantRepository) Issue(
	ctx context.Context,
	grant entity.PreviewGrant,
	ttl time.Duration,
) (string, error) {
	token, err := opaque()
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(store(grant))
	if err != nil {
		return "", fmt.Errorf("encode preview grant: %w", err)
	}

	key := grantKey(token)

	if err := r.client.Set(ctx, key, payload, ttl).Err(); err != nil {
		return "", fmt.Errorf("store preview grant: %w", err)
	}

	if grant.LinkID == uuid.Nil {
		return token, nil
	}

	linked := linkedKey(grant.LinkID)

	if err := r.client.SAdd(ctx, linked, key).Err(); err != nil {
		return "", fmt.Errorf("remember what a share link let in: %w", err)
	}

	if err := r.client.Expire(ctx, linked, ttl).Err(); err != nil {
		return "", fmt.Errorf("age what a share link let in: %w", err)
	}

	return token, nil
}

func (r *grantRepository) IssueTicket(
	ctx context.Context,
	grant entity.PreviewGrant,
	ttl time.Duration,
) (string, error) {
	ticket, err := opaque()
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(store(grant))
	if err != nil {
		return "", fmt.Errorf("encode preview ticket: %w", err)
	}

	if err := r.client.Set(ctx, ticketKey(ticket), payload, ttl).Err(); err != nil {
		return "", fmt.Errorf("store preview ticket: %w", err)
	}

	return ticket, nil
}

func (r *grantRepository) RedeemTicket(
	ctx context.Context,
	ticket string,
) (entity.PreviewGrant, error) {
	payload, err := r.client.GetDel(ctx, ticketKey(ticket)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.PreviewGrant{}, entity.ErrPreviewGrantNotFound
		}

		return entity.PreviewGrant{}, fmt.Errorf("redeem preview ticket: %w", err)
	}

	return restore(payload)
}

func (r *grantRepository) RevokeLink(ctx context.Context, linkID uuid.UUID) error {
	linked := linkedKey(linkID)

	held, err := r.client.SMembers(ctx, linked).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("read what a share link let in: %w", err)
	}

	if len(held) > 0 {
		if err := r.client.Del(ctx, held...).Err(); err != nil {
			return fmt.Errorf("shut out what a share link let in: %w", err)
		}
	}

	if err := r.client.Del(ctx, linked).Err(); err != nil {
		return fmt.Errorf("forget what a share link let in: %w", err)
	}

	return nil
}

func (r *grantRepository) Read(
	ctx context.Context,
	token string,
) (entity.PreviewGrant, error) {
	payload, err := r.client.Get(ctx, grantKey(token)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.PreviewGrant{}, entity.ErrPreviewGrantNotFound
		}

		return entity.PreviewGrant{}, fmt.Errorf("read preview grant: %w", err)
	}

	return restore(payload)
}

func store(grant entity.PreviewGrant) storedGrant {
	return storedGrant{
		Audience:    grant.Audience,
		ExecutionID: grant.ExecutionID,
		WorkspaceID: grant.WorkspaceID.String(),
		PreviewID:   grant.PreviewID.String(),
		Path:        grant.Path,
		AccountID:   grant.AccountID.String(),
		LinkID:      grant.LinkID.String(),
		IssuedAt:    grant.IssuedAt,
		ExpiresAt:   grant.ExpiresAt,
	}
}

func restore(payload []byte) (entity.PreviewGrant, error) {
	var stored storedGrant

	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.PreviewGrant{}, fmt.Errorf("decode preview grant: %w", err)
	}

	workspaceID, err := uuid.Parse(stored.WorkspaceID)
	if err != nil {
		return entity.PreviewGrant{}, fmt.Errorf("parse workspace id: %w", err)
	}

	previewID, err := uuid.Parse(stored.PreviewID)
	if err != nil {
		return entity.PreviewGrant{}, fmt.Errorf("parse preview id: %w", err)
	}

	accountID, err := uuid.Parse(stored.AccountID)
	if err != nil {
		return entity.PreviewGrant{}, fmt.Errorf("parse account id: %w", err)
	}

	linkID, err := uuid.Parse(stored.LinkID)
	if err != nil {
		return entity.PreviewGrant{}, fmt.Errorf("parse share link id: %w", err)
	}

	return entity.PreviewGrant{
		Audience:    stored.Audience,
		ExecutionID: stored.ExecutionID,
		WorkspaceID: workspaceID,
		PreviewID:   previewID,
		Path:        stored.Path,
		AccountID:   accountID,
		LinkID:      linkID,
		IssuedAt:    stored.IssuedAt,
		ExpiresAt:   stored.ExpiresAt,
	}, nil
}

func (r *grantRepository) Revoke(ctx context.Context, token string) error {
	if err := r.client.Del(ctx, grantKey(token)).Err(); err != nil {
		return fmt.Errorf("revoke preview grant: %w", err)
	}

	return nil
}

func (r *grantRepository) FirstLook(
	ctx context.Context,
	viewer string,
	window time.Duration,
) (bool, error) {
	err := r.client.SetArgs(ctx, lookKey(viewer), looked, redis.SetArgs{
		Mode: "NX",
		TTL:  window,
	}).Err()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}

		return false, fmt.Errorf("remember who looked at a preview: %w", err)
	}

	return true, nil
}

func (r *grantRepository) Attempt(
	ctx context.Context,
	subject string,
	window time.Duration,
) (int, error) {
	key := attemptKey(subject)

	made, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("count a share link attempt: %w", err)
	}

	if made == 1 {
		if err := r.client.Expire(ctx, key, window).Err(); err != nil {
			return 0, fmt.Errorf("age a share link attempt: %w", err)
		}
	}

	return int(made), nil
}

func (r *grantRepository) GrantGateway(
	ctx context.Context,
	tokenHash []byte,
	gateway entity.PreviewGateway,
	ttl time.Duration,
) error {
	payload, err := json.Marshal(storedGateway{ID: gateway.ID.String(), Name: gateway.Name})
	if err != nil {
		return fmt.Errorf("encode preview gateway access: %w", err)
	}

	key := gatewayKey(tokenHash)

	if err := r.client.Set(ctx, key, payload, ttl).Err(); err != nil {
		return fmt.Errorf("grant preview gateway access: %w", err)
	}

	holding := holdingKey(gateway.ID)

	if err := r.client.SAdd(ctx, holding, key).Err(); err != nil {
		return fmt.Errorf("remember a preview gateway credential: %w", err)
	}

	if err := r.client.Expire(ctx, holding, ttl).Err(); err != nil {
		return fmt.Errorf("age a preview gateway credential: %w", err)
	}

	return nil
}

func (r *grantRepository) ResolveGateway(
	ctx context.Context,
	tokenHash []byte,
) (entity.PreviewGateway, error) {
	payload, err := r.client.Get(ctx, gatewayKey(tokenHash)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.PreviewGateway{}, entity.ErrPreviewGatewayCredentialInvalid
		}

		return entity.PreviewGateway{}, fmt.Errorf("read preview gateway access: %w", err)
	}

	var stored storedGateway
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.PreviewGateway{}, fmt.Errorf("decode preview gateway access: %w", err)
	}

	gatewayID, err := uuid.Parse(stored.ID)
	if err != nil {
		return entity.PreviewGateway{}, fmt.Errorf("parse preview gateway id: %w", err)
	}

	return entity.PreviewGateway{
		ID:     gatewayID,
		Name:   stored.Name,
		Status: entity.PreviewGatewayActive,
	}, nil
}

func (r *grantRepository) RevokeGateway(ctx context.Context, gatewayID uuid.UUID) error {
	holding := holdingKey(gatewayID)

	held, err := r.client.SMembers(ctx, holding).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("read what a preview gateway holds: %w", err)
	}

	if len(held) > 0 {
		if err := r.client.Del(ctx, held...).Err(); err != nil {
			return fmt.Errorf("revoke preview gateway access: %w", err)
		}
	}

	if err := r.client.Del(ctx, holding).Err(); err != nil {
		return fmt.Errorf("forget what a preview gateway held: %w", err)
	}

	return nil
}

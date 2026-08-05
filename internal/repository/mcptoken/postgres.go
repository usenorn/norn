package mcptoken

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const tokenColumns = `
	t.id, t.connection_id, t.kind, t.token_hash, t.expires_at, t.consumed_at, t.created_at`

type mcpTokenRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.MCPToken {
	return &mcpTokenRepository{db: db}
}

func (r *mcpTokenRepository) Create(ctx context.Context, token entity.MCPToken) (entity.MCPToken, error) {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`INSERT INTO mcp_tokens (id, connection_id, kind, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		token.ID.String(),
		token.ConnectionID.String(),
		string(token.Kind),
		token.TokenHash,
		token.ExpiresAt,
	); err != nil {
		return entity.MCPToken{}, fmt.Errorf("insert mcp token: %w", err)
	}

	return r.getByID(ctx, token.ID)
}

func (r *mcpTokenRepository) GetByHash(ctx context.Context, tokenHash []byte) (entity.MCPToken, error) {
	return r.one(
		ctx,
		`SELECT`+tokenColumns+` FROM mcp_tokens t WHERE t.token_hash = $1`,
		tokenHash,
	)
}

func (r *mcpTokenRepository) Consume(ctx context.Context, tokenID uuid.UUID, consumedAt time.Time) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE mcp_tokens SET consumed_at = $2 WHERE id = $1 AND consumed_at IS NULL`,
		tokenID.String(),
		consumedAt,
	)
	if err != nil {
		return fmt.Errorf("consume mcp token: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("consume mcp token: %w", err)
	}

	if affected == 0 {
		return entity.ErrMCPTokenNotFound
	}

	return nil
}

func (r *mcpTokenRepository) DeleteForConnection(ctx context.Context, connectionID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`DELETE FROM mcp_tokens WHERE connection_id = $1`,
		connectionID.String(),
	); err != nil {
		return fmt.Errorf("delete mcp tokens for connection: %w", err)
	}

	return nil
}

func (r *mcpTokenRepository) PruneExpired(
	ctx context.Context,
	connectionID uuid.UUID,
	before time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`DELETE FROM mcp_tokens WHERE connection_id = $1 AND expires_at <= $2`,
		connectionID.String(),
		before,
	); err != nil {
		return fmt.Errorf("prune expired mcp tokens: %w", err)
	}

	return nil
}

func (r *mcpTokenRepository) getByID(ctx context.Context, tokenID uuid.UUID) (entity.MCPToken, error) {
	return r.one(
		ctx,
		`SELECT`+tokenColumns+` FROM mcp_tokens t WHERE t.id = $1`,
		tokenID.String(),
	)
}

func (r *mcpTokenRepository) one(
	ctx context.Context,
	query string,
	args ...any,
) (entity.MCPToken, error) {
	var (
		rawID, rawConnection string
		kind                 string
		tokenHash            []byte
		expiresAt, createdAt time.Time
		consumedAt           sql.NullTime
	)

	err := r.db.Querier(ctx).QueryRowContext(ctx, query, args...).Scan(
		&rawID, &rawConnection, &kind, &tokenHash, &expiresAt, &consumedAt, &createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.MCPToken{}, entity.ErrMCPTokenNotFound
		}

		return entity.MCPToken{}, fmt.Errorf("query mcp token: %w", err)
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return entity.MCPToken{}, fmt.Errorf("parse mcp token id: %w", err)
	}

	connectionID, err := uuid.Parse(rawConnection)
	if err != nil {
		return entity.MCPToken{}, fmt.Errorf("parse mcp token connection id: %w", err)
	}

	token := entity.MCPToken{
		ID:           id,
		ConnectionID: connectionID,
		Kind:         entity.MCPTokenKind(kind),
		TokenHash:    tokenHash,
		ExpiresAt:    expiresAt,
		CreatedAt:    createdAt,
	}

	if consumedAt.Valid {
		token.ConsumedAt = &consumedAt.Time
	}

	return token, nil
}

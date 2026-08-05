package mcpclient

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type mcpClientRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.MCPClient {
	return &mcpClientRepository{db: db}
}

func (r *mcpClientRepository) Create(ctx context.Context, client entity.MCPClient) (entity.MCPClient, error) {
	if client.ID == uuid.Nil {
		client.ID = uuid.New()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`INSERT INTO mcp_clients (id, name, redirect_uris) VALUES ($1, $2, $3)`,
		client.ID.String(),
		client.Name,
		types.StringArray(client.RedirectURIs),
	); err != nil {
		return entity.MCPClient{}, fmt.Errorf("insert mcp client: %w", err)
	}

	return r.GetByID(ctx, client.ID)
}

func (r *mcpClientRepository) GetByID(ctx context.Context, clientID uuid.UUID) (entity.MCPClient, error) {
	var (
		rawID        string
		name         string
		redirectURIs types.StringArray
		createdAt    time.Time
	)

	err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		`SELECT id, name, redirect_uris, created_at FROM mcp_clients WHERE id = $1`,
		clientID.String(),
	).Scan(&rawID, &name, &redirectURIs, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.MCPClient{}, entity.ErrMCPClientNotFound
		}

		return entity.MCPClient{}, fmt.Errorf("query mcp client: %w", err)
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return entity.MCPClient{}, fmt.Errorf("parse mcp client id: %w", err)
	}

	return entity.MCPClient{
		ID:           id,
		Name:         name,
		RedirectURIs: redirectURIs,
		CreatedAt:    createdAt,
	}, nil
}

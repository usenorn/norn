package previewgateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	nameUniqueIndex     = "preview_gateways_name_key"
)

const gatewayColumns = `
       id,
       name,
       status,
       last_seen_at,
       created_at,
       updated_at`

const createGatewayQuery = `
WITH created AS (
    INSERT INTO preview_gateways (name, secret_hash, status)
    VALUES ($1, $2, $3)
    RETURNING *
)
SELECT` + gatewayColumns + `
FROM created`

const adoptGatewayQuery = `
WITH adopted AS (
    INSERT INTO preview_gateways (name, secret_hash, status)
    VALUES ($1, $2, $3)
    ON CONFLICT (lower(name)) DO UPDATE
        SET secret_hash = excluded.secret_hash,
            status      = excluded.status,
            updated_at  = now()
    RETURNING *
)
SELECT` + gatewayColumns + `
FROM adopted`

const gatewayByCredentialQuery = `
SELECT` + gatewayColumns + `
FROM preview_gateways
WHERE secret_hash = $1`

const gatewaysQuery = `
SELECT` + gatewayColumns + `
FROM preview_gateways
ORDER BY name, id`

const revokeGatewayQuery = `
WITH revoked AS (
    UPDATE preview_gateways
    SET status = 'revoked', updated_at = now()
    WHERE id = $1
    RETURNING *
)
SELECT` + gatewayColumns + `
FROM revoked`

const gatewaySeenQuery = `
UPDATE preview_gateways
SET last_seen_at = $2, updated_at = now()
WHERE id = $1`

type gatewayRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.PreviewGateway {
	return &gatewayRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *gatewayRepository) Create(
	ctx context.Context,
	gateway entity.PreviewGateway,
	secretHash []byte,
) (entity.PreviewGateway, error) {
	created, err := scanGateway(r.db.Querier(ctx).QueryRowContext(
		ctx, createGatewayQuery, gateway.Name, secretHash, string(entity.PreviewGatewayActive),
	))

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == uniqueViolationCode &&
			pgErr.ConstraintName == nameUniqueIndex {
			return entity.PreviewGateway{}, entity.ErrPreviewGatewayNameTaken
		}

		return entity.PreviewGateway{}, fmt.Errorf("create a preview gateway: %w", err)
	}

	return created, nil
}

func (r *gatewayRepository) Adopt(
	ctx context.Context,
	name string,
	secretHash []byte,
) (entity.PreviewGateway, error) {
	adopted, err := scanGateway(r.db.Querier(ctx).QueryRowContext(
		ctx, adoptGatewayQuery, name, secretHash, string(entity.PreviewGatewayActive),
	))
	if err != nil {
		return entity.PreviewGateway{}, fmt.Errorf("adopt a preview gateway: %w", err)
	}

	return adopted, nil
}

func (r *gatewayRepository) ByCredential(
	ctx context.Context,
	secretHash []byte,
) (entity.PreviewGateway, error) {
	stored, err := scanGateway(r.db.Querier(ctx).QueryRowContext(
		ctx, gatewayByCredentialQuery, secretHash,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return entity.PreviewGateway{}, entity.ErrPreviewGatewayNotFound
	}

	if err != nil {
		return entity.PreviewGateway{}, fmt.Errorf("read a preview gateway: %w", err)
	}

	return stored, nil
}

func (r *gatewayRepository) List(ctx context.Context) ([]entity.PreviewGateway, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, gatewaysQuery)
	if err != nil {
		return nil, fmt.Errorf("list preview gateways: %w", err)
	}

	defer func() { _ = rows.Close() }()

	gateways := make([]entity.PreviewGateway, 0)

	for rows.Next() {
		gateway, err := scanGateway(rows)
		if err != nil {
			return nil, fmt.Errorf("read a preview gateway: %w", err)
		}

		gateways = append(gateways, gateway)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read preview gateways: %w", err)
	}

	return gateways, nil
}

func (r *gatewayRepository) Revoke(
	ctx context.Context,
	gatewayID uuid.UUID,
) (entity.PreviewGateway, error) {
	revoked, err := scanGateway(r.db.Querier(ctx).QueryRowContext(
		ctx, revokeGatewayQuery, gatewayID.String(),
	))

	if errors.Is(err, sql.ErrNoRows) {
		return entity.PreviewGateway{}, entity.ErrPreviewGatewayNotFound
	}

	if err != nil {
		return entity.PreviewGateway{}, fmt.Errorf("revoke a preview gateway: %w", err)
	}

	return revoked, nil
}

func (r *gatewayRepository) Seen(
	ctx context.Context,
	gatewayID uuid.UUID,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, gatewaySeenQuery, gatewayID.String(), at,
	); err != nil {
		return fmt.Errorf("record that a preview gateway called: %w", err)
	}

	return nil
}

func scanGateway(row scanner) (entity.PreviewGateway, error) {
	var (
		gateway    entity.PreviewGateway
		id         string
		status     string
		lastSeenAt sql.NullTime
	)

	if err := row.Scan(
		&id,
		&gateway.Name,
		&status,
		&lastSeenAt,
		&gateway.CreatedAt,
		&gateway.UpdatedAt,
	); err != nil {
		return entity.PreviewGateway{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.PreviewGateway{}, fmt.Errorf("parse preview gateway id: %w", err)
	}

	gateway.ID = parsed
	gateway.Status = entity.PreviewGatewayStatus(status)

	if lastSeenAt.Valid {
		gateway.LastSeenAt = lastSeenAt.Time
	}

	return gateway, nil
}

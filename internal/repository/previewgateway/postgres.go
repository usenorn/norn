package previewgateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
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

const revokeGatewayQuery = `
WITH revoked AS (
    UPDATE preview_gateways
    SET status = 'revoked', updated_at = now()
    WHERE id = $1
    RETURNING *
)
SELECT` + gatewayColumns + `
FROM revoked`

func gatewayOf(model *dbpostgres.PreviewGateway) (entity.PreviewGateway, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.PreviewGateway{}, fmt.Errorf("parse preview gateway id: %w", err)
	}

	gateway := entity.PreviewGateway{
		ID:        id,
		Name:      model.Name,
		Status:    entity.PreviewGatewayStatus(model.Status),
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}

	if model.LastSeenAt.Valid {
		gateway.LastSeenAt = model.LastSeenAt.Time
	}

	return gateway, nil
}

func gatewaysOf(models dbpostgres.PreviewGatewaySlice) ([]entity.PreviewGateway, error) {
	gateways := make([]entity.PreviewGateway, 0, len(models))

	for _, model := range models {
		gateway, err := gatewayOf(model)
		if err != nil {
			return nil, err
		}

		gateways = append(gateways, gateway)
	}

	return gateways, nil
}

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
	model := &dbpostgres.PreviewGateway{
		Name:       gateway.Name,
		SecretHash: secretHash,
		Status:     string(entity.PreviewGatewayActive),
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == uniqueViolationCode &&
			pgErr.ConstraintName == nameUniqueIndex {
			return entity.PreviewGateway{}, entity.ErrPreviewGatewayNameTaken
		}

		return entity.PreviewGateway{}, fmt.Errorf("create a preview gateway: %w", err)
	}

	return gatewayOf(model)
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
	model, err := dbpostgres.PreviewGateways(
		dbpostgres.PreviewGatewayWhere.SecretHash.EQ(secretHash),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.PreviewGateway{}, entity.ErrPreviewGatewayNotFound
		}

		return entity.PreviewGateway{}, fmt.Errorf("read a preview gateway: %w", err)
	}

	return gatewayOf(model)
}

func (r *gatewayRepository) List(ctx context.Context) ([]entity.PreviewGateway, error) {
	models, err := dbpostgres.PreviewGateways(
		qm.OrderBy(
			dbpostgres.PreviewGatewayColumns.Name+", "+dbpostgres.PreviewGatewayColumns.ID,
		),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list preview gateways: %w", err)
	}

	return gatewaysOf(models)
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
	if _, err := dbpostgres.PreviewGateways(
		dbpostgres.PreviewGatewayWhere.ID.EQ(gatewayID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.PreviewGatewayColumns.LastSeenAt: at,
		dbpostgres.PreviewGatewayColumns.UpdatedAt:  time.Now().UTC(),
	}); err != nil {
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

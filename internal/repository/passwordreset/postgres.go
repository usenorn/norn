package passwordreset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

func toEntity(model *dbpostgres.AccountPasswordReset) (entity.PasswordReset, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.PasswordReset{}, fmt.Errorf("parse password reset id: %w", err)
	}

	accountID, err := uuid.Parse(model.AccountID)
	if err != nil {
		return entity.PasswordReset{}, fmt.Errorf("parse password reset account id: %w", err)
	}

	reset := entity.PasswordReset{
		ID:          id,
		AccountID:   accountID,
		TokenHash:   model.TokenHash,
		RequestedAt: model.RequestedAt,
		ExpiresAt:   model.ExpiresAt,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}

	if model.UsedAt.Valid {
		usedAt := model.UsedAt.Time
		reset.UsedAt = &usedAt
	}

	return reset, nil
}

func toModel(reset entity.PasswordReset) *dbpostgres.AccountPasswordReset {
	model := &dbpostgres.AccountPasswordReset{
		ID:          reset.ID.String(),
		AccountID:   reset.AccountID.String(),
		TokenHash:   reset.TokenHash,
		RequestedAt: reset.RequestedAt,
		ExpiresAt:   reset.ExpiresAt,
		CreatedAt:   reset.CreatedAt,
		UpdatedAt:   reset.UpdatedAt,
	}

	if reset.UsedAt != nil {
		model.UsedAt = null.TimeFrom(*reset.UsedAt)
	}

	return model
}

type passwordResetRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.PasswordReset {
	return &passwordResetRepository{db: db}
}

func (r *passwordResetRepository) Create(ctx context.Context, reset entity.PasswordReset) (entity.PasswordReset, error) {
	if reset.ID == uuid.Nil {
		reset.ID = uuid.New()
	}

	now := time.Now().UTC()
	reset.CreatedAt = now
	reset.UpdatedAt = now

	model := toModel(reset)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return entity.PasswordReset{}, fmt.Errorf("insert password reset: %w", err)
	}

	return toEntity(model)
}

func (r *passwordResetRepository) GetByID(ctx context.Context, id uuid.UUID) (entity.PasswordReset, error) {
	model, err := dbpostgres.FindAccountPasswordReset(ctx, r.db.Querier(ctx), id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.PasswordReset{}, entity.ErrPasswordResetNotFound
		}

		return entity.PasswordReset{}, fmt.Errorf("find password reset by id: %w", err)
	}

	return toEntity(model)
}

func (r *passwordResetRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.PasswordReset, error) {
	model, err := dbpostgres.AccountPasswordResets(
		dbpostgres.AccountPasswordResetWhere.TokenHash.EQ(tokenHash),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.PasswordReset{}, entity.ErrPasswordResetNotFound
		}

		return entity.PasswordReset{}, fmt.Errorf("find password reset by token: %w", err)
	}

	return toEntity(model)
}

func (r *passwordResetRepository) MarkUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	updated, err := dbpostgres.AccountPasswordResets(
		dbpostgres.AccountPasswordResetWhere.ID.EQ(id.String()),
		qm.Where("used_at IS NULL"),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.AccountPasswordResetColumns.UsedAt:    null.TimeFrom(usedAt),
		dbpostgres.AccountPasswordResetColumns.UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("mark password reset used: %w", err)
	}

	if updated == 0 {
		return entity.ErrPasswordResetAlreadyUsed
	}

	return nil
}

func (r *passwordResetRepository) DeletePendingByAccountID(ctx context.Context, accountID uuid.UUID) error {
	if _, err := dbpostgres.AccountPasswordResets(
		dbpostgres.AccountPasswordResetWhere.AccountID.EQ(accountID.String()),
		qm.Where("used_at IS NULL"),
	).DeleteAll(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("delete pending password resets: %w", err)
	}

	return nil
}

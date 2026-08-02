package emailchange

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
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	pendingUniqueIndex  = "account_email_changes_pending_key"
)

func toEntity(model *dbpostgres.AccountEmailChange) (entity.EmailChange, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.EmailChange{}, fmt.Errorf("parse email change id: %w", err)
	}

	accountID, err := uuid.Parse(model.AccountID)
	if err != nil {
		return entity.EmailChange{}, fmt.Errorf("parse email change account id: %w", err)
	}

	change := entity.EmailChange{
		ID:          id,
		AccountID:   accountID,
		NewEmail:    model.NewEmail,
		TokenHash:   model.TokenHash,
		RequestedAt: model.RequestedAt,
		ExpiresAt:   model.ExpiresAt,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}

	if model.ConfirmedAt.Valid {
		confirmedAt := model.ConfirmedAt.Time
		change.ConfirmedAt = &confirmedAt
	}

	return change, nil
}

func toModel(change entity.EmailChange) *dbpostgres.AccountEmailChange {
	model := &dbpostgres.AccountEmailChange{
		ID:          change.ID.String(),
		AccountID:   change.AccountID.String(),
		NewEmail:    change.NewEmail,
		TokenHash:   change.TokenHash,
		RequestedAt: change.RequestedAt,
		ExpiresAt:   change.ExpiresAt,
		CreatedAt:   change.CreatedAt,
		UpdatedAt:   change.UpdatedAt,
	}

	if change.ConfirmedAt != nil {
		model.ConfirmedAt = null.TimeFrom(*change.ConfirmedAt)
	}

	return model
}

type emailChangeRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.EmailChange {
	return &emailChangeRepository{db: db}
}

func (r *emailChangeRepository) Create(ctx context.Context, change entity.EmailChange) (entity.EmailChange, error) {
	if change.ID == uuid.Nil {
		change.ID = uuid.New()
	}

	now := time.Now().UTC()
	change.CreatedAt = now
	change.UpdatedAt = now

	model := toModel(change)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == pendingUniqueIndex {
			return entity.EmailChange{}, entity.ErrEmailChangePending
		}

		return entity.EmailChange{}, fmt.Errorf("insert email change: %w", err)
	}

	return toEntity(model)
}

func (r *emailChangeRepository) GetByID(ctx context.Context, id uuid.UUID) (entity.EmailChange, error) {
	model, err := dbpostgres.FindAccountEmailChange(ctx, r.db.Querier(ctx), id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.EmailChange{}, entity.ErrEmailChangeNotFound
		}

		return entity.EmailChange{}, fmt.Errorf("find email change by id: %w", err)
	}

	return toEntity(model)
}

func (r *emailChangeRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.EmailChange, error) {
	model, err := dbpostgres.AccountEmailChanges(
		dbpostgres.AccountEmailChangeWhere.TokenHash.EQ(tokenHash),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.EmailChange{}, entity.ErrEmailChangeNotFound
		}

		return entity.EmailChange{}, fmt.Errorf("find email change by token: %w", err)
	}

	return toEntity(model)
}

func (r *emailChangeRepository) GetPendingByAccountID(ctx context.Context, accountID uuid.UUID) (entity.EmailChange, error) {
	model, err := dbpostgres.AccountEmailChanges(
		dbpostgres.AccountEmailChangeWhere.AccountID.EQ(accountID.String()),
		qm.Where("confirmed_at IS NULL"),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.EmailChange{}, entity.ErrEmailChangeNotFound
		}

		return entity.EmailChange{}, fmt.Errorf("find pending email change: %w", err)
	}

	return toEntity(model)
}

func (r *emailChangeRepository) MarkConfirmed(ctx context.Context, id uuid.UUID, confirmedAt time.Time) error {
	updated, err := dbpostgres.AccountEmailChanges(
		dbpostgres.AccountEmailChangeWhere.ID.EQ(id.String()),
		qm.Where("confirmed_at IS NULL"),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.AccountEmailChangeColumns.ConfirmedAt: null.TimeFrom(confirmedAt),
		dbpostgres.AccountEmailChangeColumns.UpdatedAt:   time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("mark email change confirmed: %w", err)
	}

	if updated == 0 {
		return entity.ErrEmailChangeNotFound
	}

	return nil
}

func (r *emailChangeRepository) DeletePendingByAccountID(ctx context.Context, accountID uuid.UUID) error {
	if _, err := dbpostgres.AccountEmailChanges(
		dbpostgres.AccountEmailChangeWhere.AccountID.EQ(accountID.String()),
		qm.Where("confirmed_at IS NULL"),
	).DeleteAll(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("delete pending email changes: %w", err)
	}

	return nil
}

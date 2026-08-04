package signup

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

func toEntity(model *dbpostgres.AccountSignUp) (entity.SignUp, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.SignUp{}, fmt.Errorf("parse sign-up id: %w", err)
	}

	signUp := entity.SignUp{
		ID:           id,
		Email:        model.Email,
		DisplayName:  model.DisplayName,
		Timezone:     model.Timezone,
		PasswordHash: model.PasswordHash,
		TokenHash:    model.TokenHash,
		RequestedAt:  model.RequestedAt,
		ExpiresAt:    model.ExpiresAt,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}

	if model.ConfirmedAt.Valid {
		confirmedAt := model.ConfirmedAt.Time
		signUp.ConfirmedAt = &confirmedAt
	}

	return signUp, nil
}

func toModel(signUp entity.SignUp) *dbpostgres.AccountSignUp {
	model := &dbpostgres.AccountSignUp{
		ID:           signUp.ID.String(),
		Email:        signUp.Email,
		DisplayName:  signUp.DisplayName,
		Timezone:     signUp.Timezone,
		PasswordHash: signUp.PasswordHash,
		TokenHash:    signUp.TokenHash,
		RequestedAt:  signUp.RequestedAt,
		ExpiresAt:    signUp.ExpiresAt,
		CreatedAt:    signUp.CreatedAt,
		UpdatedAt:    signUp.UpdatedAt,
	}

	if signUp.ConfirmedAt != nil {
		model.ConfirmedAt = null.TimeFrom(*signUp.ConfirmedAt)
	}

	return model
}

type signUpRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.SignUp {
	return &signUpRepository{db: db}
}

func (r *signUpRepository) Create(ctx context.Context, signUp entity.SignUp) (entity.SignUp, error) {
	if signUp.ID == uuid.Nil {
		signUp.ID = uuid.New()
	}

	now := time.Now().UTC()
	signUp.CreatedAt = now
	signUp.UpdatedAt = now

	model := toModel(signUp)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return entity.SignUp{}, fmt.Errorf("insert sign-up: %w", err)
	}

	return toEntity(model)
}

func (r *signUpRepository) GetByID(ctx context.Context, id uuid.UUID) (entity.SignUp, error) {
	model, err := dbpostgres.FindAccountSignUp(ctx, r.db.Querier(ctx), id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.SignUp{}, entity.ErrSignUpNotFound
		}

		return entity.SignUp{}, fmt.Errorf("find sign-up by id: %w", err)
	}

	return toEntity(model)
}

func (r *signUpRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.SignUp, error) {
	model, err := dbpostgres.AccountSignUps(
		dbpostgres.AccountSignUpWhere.TokenHash.EQ(tokenHash),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.SignUp{}, entity.ErrSignUpNotFound
		}

		return entity.SignUp{}, fmt.Errorf("find sign-up by token: %w", err)
	}

	return toEntity(model)
}

func (r *signUpRepository) MarkConfirmed(ctx context.Context, id uuid.UUID, confirmedAt time.Time) error {
	updated, err := dbpostgres.AccountSignUps(
		dbpostgres.AccountSignUpWhere.ID.EQ(id.String()),
		qm.Where("confirmed_at IS NULL"),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.AccountSignUpColumns.ConfirmedAt: null.TimeFrom(confirmedAt),
		dbpostgres.AccountSignUpColumns.UpdatedAt:   time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("mark sign-up confirmed: %w", err)
	}

	if updated == 0 {
		return entity.ErrSignUpAlreadyConfirmed
	}

	return nil
}

func (r *signUpRepository) DeletePendingByEmail(ctx context.Context, email string) error {
	if _, err := dbpostgres.AccountSignUps(
		dbpostgres.AccountSignUpWhere.Email.EQ(email),
		qm.Where("confirmed_at IS NULL"),
	).DeleteAll(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("delete pending sign-ups: %w", err)
	}

	return nil
}

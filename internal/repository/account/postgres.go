package account

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
	emailUniqueIndex    = "accounts_email_lower_key"
)

func toEntity(model *dbpostgres.Account) (entity.Account, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.Account{}, fmt.Errorf("parse account id: %w", err)
	}

	account := entity.Account{
		ID:              id,
		Status:          entity.AccountStatus(model.Status),
		Kind:            entity.AccountKind(model.Kind),
		Email:           model.Email.String,
		DisplayName:     model.DisplayName.String,
		AvatarObjectKey: model.AvatarObjectKey.String,
		Timezone:        model.Timezone.String,
		PasswordHash:    model.PasswordHash.String,
		InstanceAdmin:   model.InstanceAdmin,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}

	if model.DeactivatedAt.Valid {
		deactivatedAt := model.DeactivatedAt.Time
		account.DeactivatedAt = &deactivatedAt
	}

	if model.DeletedAt.Valid {
		deletedAt := model.DeletedAt.Time
		account.DeletedAt = &deletedAt
	}

	return account, nil
}

func kindOrPerson(kind entity.AccountKind) string {
	if kind == "" {
		return string(entity.AccountKindPerson)
	}

	return string(kind)
}

func toModel(account entity.Account) *dbpostgres.Account {
	model := &dbpostgres.Account{
		ID:              account.ID.String(),
		Status:          string(account.Status),
		Kind:            kindOrPerson(account.Kind),
		Email:           null.NewString(account.Email, account.Email != ""),
		DisplayName:     null.NewString(account.DisplayName, account.DisplayName != ""),
		AvatarObjectKey: null.NewString(account.AvatarObjectKey, account.AvatarObjectKey != ""),
		Timezone:        null.NewString(account.Timezone, account.Timezone != ""),
		PasswordHash:    null.NewString(account.PasswordHash, account.PasswordHash != ""),
		InstanceAdmin:   account.InstanceAdmin,
		CreatedAt:       account.CreatedAt,
		UpdatedAt:       account.UpdatedAt,
	}

	if account.DeactivatedAt != nil {
		model.DeactivatedAt = null.TimeFrom(*account.DeactivatedAt)
	}

	if account.DeletedAt != nil {
		model.DeletedAt = null.TimeFrom(*account.DeletedAt)
	}

	return model
}

func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == emailUniqueIndex {
		return entity.ErrAccountEmailTaken
	}

	return err
}

type accountRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Account {
	return &accountRepository{db: db}
}

func (r *accountRepository) Create(ctx context.Context, account entity.Account) (entity.Account, error) {
	if account.ID == uuid.Nil {
		account.ID = uuid.New()
	}

	now := time.Now().UTC()
	account.CreatedAt = now
	account.UpdatedAt = now

	model := toModel(account)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		if translated := translateWriteError(err); errors.Is(translated, entity.ErrAccountEmailTaken) {
			return entity.Account{}, translated
		}

		return entity.Account{}, fmt.Errorf("insert account: %w", err)
	}

	return toEntity(model)
}

func (r *accountRepository) GetByID(ctx context.Context, id uuid.UUID) (entity.Account, error) {
	model, err := dbpostgres.FindAccount(ctx, r.db.Querier(ctx), id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Account{}, entity.ErrAccountNotFound
		}

		return entity.Account{}, fmt.Errorf("find account by id: %w", err)
	}

	return toEntity(model)
}

func (r *accountRepository) GetByEmail(ctx context.Context, email string) (entity.Account, error) {
	model, err := dbpostgres.Accounts(
		qm.Where("lower(email) = lower(?)", email),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Account{}, entity.ErrAccountNotFound
		}

		return entity.Account{}, fmt.Errorf("find account by email: %w", err)
	}

	return toEntity(model)
}

func (r *accountRepository) ListByIDs(ctx context.Context, ids []uuid.UUID) ([]entity.Account, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = id.String()
	}

	models, err := dbpostgres.Accounts(
		dbpostgres.AccountWhere.ID.IN(keys),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list accounts by ids: %w", err)
	}

	accounts := make([]entity.Account, 0, len(models))

	for _, model := range models {
		account, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		accounts = append(accounts, account)
	}

	return accounts, nil
}

func (r *accountRepository) Update(ctx context.Context, account entity.Account) (entity.Account, error) {
	account.UpdatedAt = time.Now().UTC()

	model := toModel(account)

	updated, err := model.Update(ctx, r.db.Querier(ctx), boil.Blacklist(
		dbpostgres.AccountColumns.ID,
		dbpostgres.AccountColumns.CreatedAt,
	))
	if err != nil {
		if translated := translateWriteError(err); errors.Is(translated, entity.ErrAccountEmailTaken) {
			return entity.Account{}, translated
		}

		return entity.Account{}, fmt.Errorf("update account: %w", err)
	}

	if updated == 0 {
		return entity.Account{}, entity.ErrAccountNotFound
	}

	return toEntity(model)
}

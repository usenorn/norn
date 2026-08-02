package passwordhistory

import (
	"context"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const pruneHistoryQuery = `
DELETE FROM account_password_history
WHERE account_id = $1
  AND id NOT IN (
      SELECT id
      FROM account_password_history
      WHERE account_id = $1
      ORDER BY created_at DESC
      LIMIT $2
  )`

func toEntity(model *dbpostgres.AccountPasswordHistory) (entity.PasswordHistoryEntry, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.PasswordHistoryEntry{}, fmt.Errorf("parse password history id: %w", err)
	}

	accountID, err := uuid.Parse(model.AccountID)
	if err != nil {
		return entity.PasswordHistoryEntry{}, fmt.Errorf("parse password history account id: %w", err)
	}

	return entity.PasswordHistoryEntry{
		ID:           id,
		AccountID:    accountID,
		PasswordHash: model.PasswordHash,
		CreatedAt:    model.CreatedAt,
	}, nil
}

type passwordHistoryRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.PasswordHistory {
	return &passwordHistoryRepository{db: db}
}

func (r *passwordHistoryRepository) Create(ctx context.Context, entry entity.PasswordHistoryEntry) (entity.PasswordHistoryEntry, error) {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}

	entry.CreatedAt = time.Now().UTC()

	model := &dbpostgres.AccountPasswordHistory{
		ID:           entry.ID.String(),
		AccountID:    entry.AccountID.String(),
		PasswordHash: entry.PasswordHash,
		CreatedAt:    entry.CreatedAt,
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return entity.PasswordHistoryEntry{}, fmt.Errorf("insert password history: %w", err)
	}

	return toEntity(model)
}

func (r *passwordHistoryRepository) ListRecentByAccountID(ctx context.Context, accountID uuid.UUID, limit int) ([]entity.PasswordHistoryEntry, error) {
	models, err := dbpostgres.AccountPasswordHistories(
		dbpostgres.AccountPasswordHistoryWhere.AccountID.EQ(accountID.String()),
		qm.OrderBy(dbpostgres.AccountPasswordHistoryColumns.CreatedAt+" DESC"),
		qm.Limit(limit),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list password history: %w", err)
	}

	entries := make([]entity.PasswordHistoryEntry, 0, len(models))

	for _, model := range models {
		entry, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func (r *passwordHistoryRepository) PruneByAccountID(ctx context.Context, accountID uuid.UUID, keep int) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, pruneHistoryQuery, accountID.String(), keep); err != nil {
		return fmt.Errorf("prune password history: %w", err)
	}

	return nil
}

func (r *passwordHistoryRepository) DeleteByAccountID(ctx context.Context, accountID uuid.UUID) error {
	if _, err := dbpostgres.AccountPasswordHistories(
		dbpostgres.AccountPasswordHistoryWhere.AccountID.EQ(accountID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("delete password history: %w", err)
	}

	return nil
}

package telegramupdate

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

const recordUpdateQuery = `
INSERT INTO workspace_telegram_updates (id, bot_id, update_id, payload, received_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (bot_id, update_id) DO NOTHING
RETURNING id`

const updateColumns = `
SELECT id, bot_id, update_id, payload, outcome, received_at, processed_at
FROM workspace_telegram_updates`

const updateOfQuery = updateColumns + `
WHERE bot_id = $1 AND update_id = $2`

const lockUpdateQuery = updateColumns + `
WHERE id = $1
FOR UPDATE`

const settleUpdateQuery = `
UPDATE workspace_telegram_updates
SET outcome = $2, processed_at = $3, payload = NULL
WHERE id = $1`

const sweepUpdatesQuery = `
DELETE FROM workspace_telegram_updates
WHERE id IN (
    SELECT id FROM workspace_telegram_updates
    WHERE received_at < $1 AND processed_at IS NOT NULL
    ORDER BY received_at
    LIMIT $2
)`

type telegramUpdateRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.TelegramUpdate {
	return &telegramUpdateRepository{db: db}
}

func (r *telegramUpdateRepository) Record(ctx context.Context, update entity.TelegramUpdate) (uuid.UUID, error) {
	id := update.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	receivedAt := update.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}

	var stored string

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, recordUpdateQuery, id.String(), update.BotID.String(), update.UpdateID, update.Payload, receivedAt,
	).Scan(&stored); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, entity.ErrTelegramUpdateDuplicate
		}

		return uuid.Nil, fmt.Errorf("record telegram update: %w", err)
	}

	return id, nil
}

func (r *telegramUpdateRepository) UpdateOf(
	ctx context.Context,
	botID uuid.UUID,
	updateID int64,
) (entity.TelegramUpdate, error) {
	return scanUpdate(r.db.Querier(ctx).QueryRowContext(ctx, updateOfQuery, botID.String(), updateID))
}

func (r *telegramUpdateRepository) Lock(ctx context.Context, id uuid.UUID) (entity.TelegramUpdate, error) {
	return scanUpdate(r.db.Querier(ctx).QueryRowContext(ctx, lockUpdateQuery, id.String()))
}

func (r *telegramUpdateRepository) Settle(
	ctx context.Context,
	id uuid.UUID,
	outcome entity.TelegramUpdateOutcome,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, settleUpdateQuery, id.String(), string(outcome), at); err != nil {
		return fmt.Errorf("settle telegram update: %w", err)
	}

	return nil
}

func (r *telegramUpdateRepository) Sweep(ctx context.Context, before time.Time, limit int) (int, error) {
	result, err := r.db.Querier(ctx).ExecContext(ctx, sweepUpdatesQuery, before, limit)
	if err != nil {
		return 0, fmt.Errorf("sweep telegram updates: %w", err)
	}

	swept, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count swept telegram updates: %w", err)
	}

	return int(swept), nil
}

func scanUpdate(row *sql.Row) (entity.TelegramUpdate, error) {
	var (
		update    entity.TelegramUpdate
		id, bot   string
		outcome   string
		processed sql.NullTime
	)

	if err := row.Scan(&id, &bot, &update.UpdateID, &update.Payload, &outcome, &update.ReceivedAt, &processed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.TelegramUpdate{}, entity.ErrTelegramUpdateNotFound
		}

		return entity.TelegramUpdate{}, fmt.Errorf("find telegram update: %w", err)
	}

	var err error

	if update.ID, err = uuid.Parse(id); err != nil {
		return entity.TelegramUpdate{}, fmt.Errorf("parse telegram update id: %w", err)
	}

	if update.BotID, err = uuid.Parse(bot); err != nil {
		return entity.TelegramUpdate{}, fmt.Errorf("parse telegram update bot id: %w", err)
	}

	update.Outcome = entity.TelegramUpdateOutcome(outcome)
	update.ReceivedAt = update.ReceivedAt.UTC()

	if processed.Valid {
		at := processed.Time.UTC()
		update.ProcessedAt = &at
	}

	return update, nil
}

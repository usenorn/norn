package telegrambot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	botUserIndex        = "workspace_telegram_bots_bot_user_id_key"
)

const botColumns = `
       b.id,
       b.workspace_id,
       b.agent_id,
       b.bot_user_id,
       b.bot_username,
       b.bot_name,
       b.token_hint,
       coalesce(b.connected_by_account_id::text, ''),
       coalesce(connector.display_name, ''),
       b.connected_at,
       b.updated_at`

const botFrom = `
FROM workspace_telegram_bots b
LEFT JOIN accounts connector ON connector.id = b.connected_by_account_id`

const botByAgentQuery = `SELECT` + botColumns + botFrom + `
WHERE b.workspace_id = $1 AND b.agent_id = $2`

const botByIDQuery = `SELECT` + botColumns + botFrom + `
WHERE b.id = $1`

const botByUserQuery = `SELECT` + botColumns + botFrom + `
WHERE b.bot_user_id = $1`

const botsByWorkspaceQuery = `SELECT` + botColumns + botFrom + `
WHERE b.workspace_id = $1
ORDER BY b.connected_at, b.id`

const tokenQuery = `
SELECT token_sealed FROM workspace_telegram_bots WHERE id = $1`

const secretQuery = `
SELECT webhook_secret_hash FROM workspace_telegram_bots WHERE id = $1`

const insertBotQuery = `
INSERT INTO workspace_telegram_bots
    (id, workspace_id, agent_id, bot_user_id, bot_username, bot_name, token_sealed, token_hint,
     webhook_secret_hash, connected_by_account_id, connected_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, nullif($10, '')::uuid, $11, $11)`

const rekeyBotQuery = `
UPDATE workspace_telegram_bots
SET bot_username            = $2,
    bot_name                = $3,
    token_sealed            = $4,
    token_hint              = $5,
    webhook_secret_hash     = $6,
    connected_by_account_id = nullif($7, '')::uuid,
    updated_at              = $8
WHERE id = $1`

const deleteBotQuery = `
DELETE FROM workspace_telegram_bots WHERE workspace_id = $1 AND agent_id = $2`

type telegramBotRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func New(db *postgres.Client, sealer *crypter.Crypter) repository.TelegramBot {
	return &telegramBotRepository{db: db, crypter: sealer}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanBot(row scanner) (entity.TelegramBot, error) {
	var (
		bot                                   entity.TelegramBot
		id, workspace, agent, connector, name string
	)

	if err := row.Scan(
		&id, &workspace, &agent, &bot.BotUserID, &bot.Username, &bot.Name, &bot.TokenHint,
		&connector, &name, &bot.ConnectedAt, &bot.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.TelegramBot{}, entity.ErrTelegramBotNotFound
		}

		return entity.TelegramBot{}, fmt.Errorf("scan telegram bot: %w", err)
	}

	var err error

	if bot.ID, err = uuid.Parse(id); err != nil {
		return entity.TelegramBot{}, fmt.Errorf("parse telegram bot id: %w", err)
	}

	if bot.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.TelegramBot{}, fmt.Errorf("parse telegram bot workspace id: %w", err)
	}

	if bot.AgentID, err = uuid.Parse(agent); err != nil {
		return entity.TelegramBot{}, fmt.Errorf("parse telegram bot agent id: %w", err)
	}

	if connector != "" {
		if bot.ConnectedBy, err = uuid.Parse(connector); err != nil {
			return entity.TelegramBot{}, fmt.Errorf("parse telegram bot connector id: %w", err)
		}
	}

	bot.ConnectedByName = name
	bot.ConnectedAt = bot.ConnectedAt.UTC()
	bot.UpdatedAt = bot.UpdatedAt.UTC()

	return bot, nil
}

func (r *telegramBotRepository) seal(token string) ([]byte, error) {
	sealed, err := r.crypter.Seal([]byte(token))
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return nil, entity.ErrTelegramEncryptionKeyMissing
		}

		return nil, fmt.Errorf("seal telegram bot token: %w", err)
	}

	return sealed, nil
}

func (r *telegramBotRepository) one(ctx context.Context, query string, args ...any) (entity.TelegramBot, error) {
	return scanBot(r.db.Querier(ctx).QueryRowContext(ctx, query, args...))
}

func (r *telegramBotRepository) Get(ctx context.Context, workspaceID, agentID uuid.UUID) (entity.TelegramBot, error) {
	return r.one(ctx, botByAgentQuery, workspaceID.String(), agentID.String())
}

func (r *telegramBotRepository) GetByID(ctx context.Context, botID uuid.UUID) (entity.TelegramBot, error) {
	return r.one(ctx, botByIDQuery, botID.String())
}

func (r *telegramBotRepository) GetByBotUser(ctx context.Context, botUserID int64) (entity.TelegramBot, error) {
	return r.one(ctx, botByUserQuery, botUserID)
}

func (r *telegramBotRepository) ListByWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.TelegramBot, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, botsByWorkspaceQuery, workspaceID.String())
	if err != nil {
		return nil, fmt.Errorf("list telegram bots: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var bots []entity.TelegramBot

	for rows.Next() {
		bot, err := scanBot(rows)
		if err != nil {
			return nil, err
		}

		bots = append(bots, bot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list telegram bots: %w", err)
	}

	return bots, nil
}

func (r *telegramBotRepository) Token(ctx context.Context, botID uuid.UUID) (string, error) {
	var sealed []byte

	if err := r.db.Querier(ctx).QueryRowContext(ctx, tokenQuery, botID.String()).Scan(&sealed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", entity.ErrTelegramBotNotFound
		}

		return "", fmt.Errorf("read telegram bot token: %w", err)
	}

	token, err := r.crypter.Open(sealed)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return "", entity.ErrTelegramEncryptionKeyMissing
		}

		return "", fmt.Errorf("open telegram bot token: %w", err)
	}

	return string(token), nil
}

func (r *telegramBotRepository) SecretHash(ctx context.Context, botID uuid.UUID) ([]byte, error) {
	var hash []byte

	if err := r.db.Querier(ctx).QueryRowContext(ctx, secretQuery, botID.String()).Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrTelegramBotNotFound
		}

		return nil, fmt.Errorf("read telegram webhook secret: %w", err)
	}

	return hash, nil
}

func (r *telegramBotRepository) Save(
	ctx context.Context,
	bot entity.TelegramBot,
	token string,
	secretHash []byte,
) (entity.TelegramBot, error) {
	sealed, err := r.seal(token)
	if err != nil {
		return entity.TelegramBot{}, err
	}

	id := bot.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, insertBotQuery,
		id.String(), bot.WorkspaceID.String(), bot.AgentID.String(), bot.BotUserID, bot.Username, bot.Name,
		sealed, entity.TelegramTokenHint(token), secretHash, optional(bot.ConnectedBy), time.Now().UTC(),
	); err != nil {
		if violates(err, botUserIndex) {
			return entity.TelegramBot{}, entity.ErrTelegramBotTaken
		}

		return entity.TelegramBot{}, fmt.Errorf("save telegram bot: %w", err)
	}

	return r.GetByID(ctx, id)
}

func (r *telegramBotRepository) Rekey(
	ctx context.Context,
	bot entity.TelegramBot,
	token string,
	secretHash []byte,
) (entity.TelegramBot, error) {
	sealed, err := r.seal(token)
	if err != nil {
		return entity.TelegramBot{}, err
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx, rekeyBotQuery,
		bot.ID.String(), bot.Username, bot.Name, sealed, entity.TelegramTokenHint(token), secretHash,
		optional(bot.ConnectedBy), time.Now().UTC(),
	)
	if err != nil {
		return entity.TelegramBot{}, fmt.Errorf("rekey telegram bot: %w", err)
	}

	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return entity.TelegramBot{}, entity.ErrTelegramBotNotFound
	}

	return r.GetByID(ctx, bot.ID)
}

func (r *telegramBotRepository) Delete(ctx context.Context, workspaceID, agentID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteBotQuery, workspaceID.String(), agentID.String())
	if err != nil {
		return fmt.Errorf("delete telegram bot: %w", err)
	}

	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return entity.ErrTelegramBotNotFound
	}

	return nil
}

func optional(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func violates(err error, index string) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolationCode &&
		pgErr.ConstraintName == index
}

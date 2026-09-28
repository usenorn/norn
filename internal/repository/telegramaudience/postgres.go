package telegramaudience

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

const issueCodeQuery = `
INSERT INTO workspace_telegram_link_codes (code_hash, bot_id, account_id, purpose, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6)`

const redeemCodeQuery = `
DELETE FROM workspace_telegram_link_codes
WHERE code_hash = $1 AND bot_id = $2
RETURNING bot_id, account_id, purpose, expires_at`

const pruneCodesQuery = `
DELETE FROM workspace_telegram_link_codes WHERE expires_at <= $1`

const releaseTelegramUserQuery = `
DELETE FROM workspace_telegram_accounts
WHERE bot_id = $1 AND telegram_user_id = $2 AND account_id <> $3`

const linkAccountQuery = `
INSERT INTO workspace_telegram_accounts
    (bot_id, account_id, telegram_user_id, chat_id, telegram_username, linked_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (bot_id, account_id) DO UPDATE
    SET telegram_user_id  = excluded.telegram_user_id,
        chat_id           = excluded.chat_id,
        telegram_username = excluded.telegram_username,
        linked_at         = excluded.linked_at`

const accountColumns = `
       l.bot_id,
       l.account_id,
       coalesce(a.display_name, ''),
       l.telegram_user_id,
       l.chat_id,
       l.telegram_username,
       l.linked_at
FROM workspace_telegram_accounts l
JOIN accounts a ON a.id = l.account_id`

const accountOfQuery = `SELECT` + accountColumns + `
WHERE l.bot_id = $1 AND l.telegram_user_id = $2 AND a.status = 'active'`

const accountForQuery = `SELECT` + accountColumns + `
WHERE l.bot_id = $1 AND l.account_id = $2 AND a.status = 'active'`

const accountsQuery = `SELECT` + accountColumns + `
WHERE l.bot_id = $1 AND a.status = 'active'
ORDER BY lower(coalesce(a.display_name, '')), l.account_id`

const unlinkQuery = `
DELETE FROM workspace_telegram_accounts WHERE bot_id = $1 AND account_id = $2`

const groupColumns = `
       g.id,
       g.bot_id,
       g.chat_id,
       g.title,
       coalesce(g.bound_by_account_id::text, ''),
       coalesce(a.display_name, ''),
       g.bound_at`

const groupFrom = `
FROM workspace_telegram_groups g
LEFT JOIN accounts a ON a.id = g.bound_by_account_id`

const bindGroupQuery = `
INSERT INTO workspace_telegram_groups (id, bot_id, chat_id, title, bound_by_account_id, bound_at)
VALUES ($1, $2, $3, $4, nullif($5, '')::uuid, $6)
ON CONFLICT (bot_id, chat_id) DO UPDATE
    SET title               = excluded.title,
        bound_by_account_id = excluded.bound_by_account_id,
        bound_at            = excluded.bound_at
RETURNING id`

const groupByIDQuery = `SELECT` + groupColumns + groupFrom + `
WHERE g.id = $1`

const groupByChatQuery = `SELECT` + groupColumns + groupFrom + `
WHERE g.bot_id = $1 AND g.chat_id = $2`

const groupsQuery = `SELECT` + groupColumns + groupFrom + `
WHERE g.bot_id = $1
ORDER BY g.bound_at, g.id`

const unbindQuery = `
DELETE FROM workspace_telegram_groups WHERE bot_id = $1 AND id = $2`

const unbindChatQuery = `
DELETE FROM workspace_telegram_groups WHERE bot_id = $1 AND chat_id = $2`

const moveGroupQuery = `
UPDATE workspace_telegram_groups SET chat_id = $3 WHERE bot_id = $1 AND chat_id = $2`

type telegramAudienceRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.TelegramAudience {
	return &telegramAudienceRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *telegramAudienceRepository) IssueCode(
	ctx context.Context,
	code entity.TelegramLinkCode,
	hash []byte,
) error {
	now := time.Now().UTC()

	if _, err := r.db.Querier(ctx).ExecContext(ctx, pruneCodesQuery, now); err != nil {
		return fmt.Errorf("prune telegram link codes: %w", err)
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, issueCodeQuery,
		hash, code.BotID.String(), code.AccountID.String(), string(code.Purpose), code.ExpiresAt, now,
	); err != nil {
		return fmt.Errorf("issue telegram link code: %w", err)
	}

	return nil
}

func (r *telegramAudienceRepository) Redeem(
	ctx context.Context,
	botID uuid.UUID,
	hash []byte,
	now time.Time,
) (entity.TelegramLinkCode, error) {
	var (
		code         entity.TelegramLinkCode
		bot, account string
		purpose      string
	)

	if err := r.db.Querier(ctx).QueryRowContext(ctx, redeemCodeQuery, hash, botID.String()).Scan(
		&bot, &account, &purpose, &code.ExpiresAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.TelegramLinkCode{}, entity.ErrTelegramLinkCodeInvalid
		}

		return entity.TelegramLinkCode{}, fmt.Errorf("redeem telegram link code: %w", err)
	}

	if !now.Before(code.ExpiresAt) {
		return entity.TelegramLinkCode{}, entity.ErrTelegramLinkCodeInvalid
	}

	var err error

	if code.BotID, err = uuid.Parse(bot); err != nil {
		return entity.TelegramLinkCode{}, fmt.Errorf("parse telegram link code bot id: %w", err)
	}

	if code.AccountID, err = uuid.Parse(account); err != nil {
		return entity.TelegramLinkCode{}, fmt.Errorf("parse telegram link code account id: %w", err)
	}

	code.Purpose = entity.TelegramLinkPurpose(purpose)

	return code, nil
}

func (r *telegramAudienceRepository) Link(ctx context.Context, account entity.TelegramAccount) error {
	linkedAt := account.LinkedAt
	if linkedAt.IsZero() {
		linkedAt = time.Now().UTC()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, releaseTelegramUserQuery,
		account.BotID.String(), account.TelegramUserID, account.AccountID.String(),
	); err != nil {
		return fmt.Errorf("release telegram user: %w", err)
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, linkAccountQuery,
		account.BotID.String(), account.AccountID.String(), account.TelegramUserID, account.ChatID,
		account.Username, linkedAt,
	); err != nil {
		return fmt.Errorf("link telegram account: %w", err)
	}

	return nil
}

func scanAccount(row scanner) (entity.TelegramAccount, error) {
	var (
		account     entity.TelegramAccount
		bot, holder string
	)

	if err := row.Scan(
		&bot, &holder, &account.AccountName, &account.TelegramUserID, &account.ChatID,
		&account.Username, &account.LinkedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.TelegramAccount{}, entity.ErrTelegramAccountNotLinked
		}

		return entity.TelegramAccount{}, fmt.Errorf("scan telegram account: %w", err)
	}

	var err error

	if account.BotID, err = uuid.Parse(bot); err != nil {
		return entity.TelegramAccount{}, fmt.Errorf("parse telegram account bot id: %w", err)
	}

	if account.AccountID, err = uuid.Parse(holder); err != nil {
		return entity.TelegramAccount{}, fmt.Errorf("parse telegram account id: %w", err)
	}

	account.LinkedAt = account.LinkedAt.UTC()

	return account, nil
}

func (r *telegramAudienceRepository) AccountOf(
	ctx context.Context,
	botID uuid.UUID,
	telegramUserID int64,
) (entity.TelegramAccount, error) {
	return scanAccount(r.db.Querier(ctx).QueryRowContext(ctx, accountOfQuery, botID.String(), telegramUserID))
}

func (r *telegramAudienceRepository) AccountFor(
	ctx context.Context,
	botID, accountID uuid.UUID,
) (entity.TelegramAccount, error) {
	return scanAccount(r.db.Querier(ctx).QueryRowContext(ctx, accountForQuery, botID.String(), accountID.String()))
}

func (r *telegramAudienceRepository) Accounts(
	ctx context.Context,
	botID uuid.UUID,
) ([]entity.TelegramAccount, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, accountsQuery, botID.String())
	if err != nil {
		return nil, fmt.Errorf("list telegram accounts: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var accounts []entity.TelegramAccount

	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}

		accounts = append(accounts, account)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list telegram accounts: %w", err)
	}

	return accounts, nil
}

func (r *telegramAudienceRepository) Unlink(ctx context.Context, botID, accountID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, unlinkQuery, botID.String(), accountID.String())
	if err != nil {
		return fmt.Errorf("unlink telegram account: %w", err)
	}

	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return entity.ErrTelegramAccountNotLinked
	}

	return nil
}

func scanGroup(row scanner) (entity.TelegramGroup, error) {
	var (
		group            entity.TelegramGroup
		id, bot, boundBy string
	)

	if err := row.Scan(
		&id, &bot, &group.ChatID, &group.Title, &boundBy, &group.BoundByName, &group.BoundAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.TelegramGroup{}, entity.ErrTelegramGroupNotFound
		}

		return entity.TelegramGroup{}, fmt.Errorf("scan telegram group: %w", err)
	}

	var err error

	if group.ID, err = uuid.Parse(id); err != nil {
		return entity.TelegramGroup{}, fmt.Errorf("parse telegram group id: %w", err)
	}

	if group.BotID, err = uuid.Parse(bot); err != nil {
		return entity.TelegramGroup{}, fmt.Errorf("parse telegram group bot id: %w", err)
	}

	if boundBy != "" {
		if group.BoundBy, err = uuid.Parse(boundBy); err != nil {
			return entity.TelegramGroup{}, fmt.Errorf("parse telegram group binder id: %w", err)
		}
	}

	group.BoundAt = group.BoundAt.UTC()

	return group, nil
}

func (r *telegramAudienceRepository) Bind(ctx context.Context, group entity.TelegramGroup) (entity.TelegramGroup, error) {
	boundAt := group.BoundAt
	if boundAt.IsZero() {
		boundAt = time.Now().UTC()
	}

	boundBy := ""
	if group.BoundBy != uuid.Nil {
		boundBy = group.BoundBy.String()
	}

	var id string

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, bindGroupQuery,
		uuid.New().String(), group.BotID.String(), group.ChatID, group.Title, boundBy, boundAt,
	).Scan(&id); err != nil {
		return entity.TelegramGroup{}, fmt.Errorf("bind telegram group: %w", err)
	}

	return scanGroup(r.db.Querier(ctx).QueryRowContext(ctx, groupByIDQuery, id))
}

func (r *telegramAudienceRepository) Groups(ctx context.Context, botID uuid.UUID) ([]entity.TelegramGroup, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, groupsQuery, botID.String())
	if err != nil {
		return nil, fmt.Errorf("list telegram groups: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var groups []entity.TelegramGroup

	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list telegram groups: %w", err)
	}

	return groups, nil
}

func (r *telegramAudienceRepository) Group(
	ctx context.Context,
	botID uuid.UUID,
	chatID int64,
) (entity.TelegramGroup, error) {
	return scanGroup(r.db.Querier(ctx).QueryRowContext(ctx, groupByChatQuery, botID.String(), chatID))
}

func (r *telegramAudienceRepository) Unbind(
	ctx context.Context,
	botID, groupID uuid.UUID,
) (entity.TelegramGroup, error) {
	group, err := scanGroup(r.db.Querier(ctx).QueryRowContext(ctx, groupByIDQuery, groupID.String()))
	if err != nil {
		return entity.TelegramGroup{}, err
	}

	if group.BotID != botID {
		return entity.TelegramGroup{}, entity.ErrTelegramGroupNotFound
	}

	if _, err := r.db.Querier(ctx).ExecContext(ctx, unbindQuery, botID.String(), groupID.String()); err != nil {
		return entity.TelegramGroup{}, fmt.Errorf("unbind telegram group: %w", err)
	}

	return group, nil
}

func (r *telegramAudienceRepository) UnbindChat(ctx context.Context, botID uuid.UUID, chatID int64) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, unbindChatQuery, botID.String(), chatID); err != nil {
		return fmt.Errorf("unbind telegram chat: %w", err)
	}

	return nil
}

func (r *telegramAudienceRepository) MoveGroup(
	ctx context.Context,
	botID uuid.UUID,
	fromChatID, toChatID int64,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, moveGroupQuery, botID.String(), fromChatID, toChatID); err != nil {
		return fmt.Errorf("move telegram group: %w", err)
	}

	return nil
}

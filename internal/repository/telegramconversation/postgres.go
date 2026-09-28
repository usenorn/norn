package telegramconversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const rememberQuery = `
INSERT INTO workspace_telegram_question_messages (bot_id, chat_id, message_id, question_id, sent_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING`

const questionAtQuery = `
SELECT question_id FROM workspace_telegram_question_messages
WHERE bot_id = $1 AND chat_id = $2 AND message_id = $3`

const postedQuery = `
SELECT bot_id, chat_id, message_id, question_id, settled_at IS NOT NULL
FROM workspace_telegram_question_messages
WHERE question_id = $1
ORDER BY sent_at, chat_id`

const markSettledQuery = `
UPDATE workspace_telegram_question_messages
SET settled_at = $4
WHERE bot_id = $1 AND chat_id = $2 AND message_id = $3`

const appendTurnQuery = `
INSERT INTO workspace_telegram_turns (id, bot_id, chat_id, role, text, created_at)
VALUES ($1, $2, $3, $4, $5, clock_timestamp())`

const historyQuery = `
SELECT role, text FROM workspace_telegram_turns
WHERE bot_id = $1 AND chat_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3`

const pruneQuery = `
DELETE FROM workspace_telegram_turns
WHERE bot_id = $1 AND chat_id = $2 AND id NOT IN (
    SELECT id FROM workspace_telegram_turns
    WHERE bot_id = $1 AND chat_id = $2
    ORDER BY created_at DESC, id DESC
    LIMIT $3
)`

type telegramConversationRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.TelegramConversation {
	return &telegramConversationRepository{db: db}
}

func (r *telegramConversationRepository) Remember(ctx context.Context, message entity.TelegramQuestionMessage) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, rememberQuery,
		message.BotID.String(), message.ChatID, message.MessageID, message.QuestionID.String(), time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("remember telegram question message: %w", err)
	}

	return nil
}

func (r *telegramConversationRepository) QuestionAt(
	ctx context.Context,
	botID uuid.UUID,
	chatID, messageID int64,
) (uuid.UUID, error) {
	var question string

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, questionAtQuery, botID.String(), chatID, messageID,
	).Scan(&question); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, entity.ErrIssueQuestionNotFound
		}

		return uuid.Nil, fmt.Errorf("find telegram question message: %w", err)
	}

	id, err := uuid.Parse(question)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse telegram question id: %w", err)
	}

	return id, nil
}

func (r *telegramConversationRepository) Posted(
	ctx context.Context,
	questionID uuid.UUID,
) ([]entity.TelegramQuestionMessage, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, postedQuery, questionID.String())
	if err != nil {
		return nil, fmt.Errorf("list telegram question messages: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var posted []entity.TelegramQuestionMessage

	for rows.Next() {
		var (
			message       entity.TelegramQuestionMessage
			bot, question string
		)

		if err := rows.Scan(&bot, &message.ChatID, &message.MessageID, &question, &message.Settled); err != nil {
			return nil, fmt.Errorf("scan telegram question message: %w", err)
		}

		if message.BotID, err = uuid.Parse(bot); err != nil {
			return nil, fmt.Errorf("parse telegram question message bot id: %w", err)
		}

		if message.QuestionID, err = uuid.Parse(question); err != nil {
			return nil, fmt.Errorf("parse telegram question message question id: %w", err)
		}

		posted = append(posted, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list telegram question messages: %w", err)
	}

	return posted, nil
}

func (r *telegramConversationRepository) MarkSettled(
	ctx context.Context,
	message entity.TelegramQuestionMessage,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, markSettledQuery, message.BotID.String(), message.ChatID, message.MessageID, at,
	); err != nil {
		return fmt.Errorf("settle telegram question message: %w", err)
	}

	return nil
}

func (r *telegramConversationRepository) Append(
	ctx context.Context,
	botID uuid.UUID,
	chatID int64,
	turn entity.AgentTurn,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, appendTurnQuery, uuid.New().String(), botID.String(), chatID, string(turn.Role), turn.Text,
	); err != nil {
		return fmt.Errorf("append telegram turn: %w", err)
	}

	return nil
}

func (r *telegramConversationRepository) History(
	ctx context.Context,
	botID uuid.UUID,
	chatID int64,
	limit int,
) ([]entity.AgentTurn, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, historyQuery, botID.String(), chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("read telegram history: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var turns []entity.AgentTurn

	for rows.Next() {
		var turn entity.AgentTurn

		var role string

		if err := rows.Scan(&role, &turn.Text); err != nil {
			return nil, fmt.Errorf("scan telegram turn: %w", err)
		}

		turn.Role = entity.AgentTurnRole(role)
		turns = append(turns, turn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read telegram history: %w", err)
	}

	slices.Reverse(turns)

	return turns, nil
}

func (r *telegramConversationRepository) Prune(ctx context.Context, botID uuid.UUID, chatID int64, keep int) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, pruneQuery, botID.String(), chatID, keep); err != nil {
		return fmt.Errorf("prune telegram history: %w", err)
	}

	return nil
}

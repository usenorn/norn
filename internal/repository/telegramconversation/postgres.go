package telegramconversation

import (
	"context"
	"database/sql"
	"encoding/json"
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
INSERT INTO workspace_telegram_decision_messages (
    bot_id, chat_id, message_id, kind, question_id, execution_id, plan_revision, review_heads, sent_at
)
VALUES ($1, $2, $3, $4, nullif($5, '')::uuid, nullif($6, ''), nullif($7, 0), $8::jsonb, $9)
ON CONFLICT DO NOTHING`

const decisionColumns = `
SELECT bot_id, chat_id, message_id, kind, coalesce(question_id::text, ''), coalesce(execution_id, ''),
       coalesce(plan_revision, 0), review_heads, settled_at IS NOT NULL
FROM workspace_telegram_decision_messages`

const decisionAtQuery = decisionColumns + `
WHERE bot_id = $1 AND chat_id = $2 AND message_id = $3`

const postedForQuestionQuery = decisionColumns + `
WHERE kind = 'question' AND question_id = $1
ORDER BY sent_at, chat_id`

const postedForExecutionQuery = decisionColumns + `
WHERE kind = $1 AND execution_id = $2
ORDER BY sent_at, chat_id`

const markSettledQuery = `
UPDATE workspace_telegram_decision_messages
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

func (r *telegramConversationRepository) Remember(ctx context.Context, message entity.TelegramDecisionMessage) error {
	var heads []byte

	if message.Kind == entity.TelegramDecisionReview {
		encoded, err := json.Marshal(message.ReviewHeads)
		if err != nil {
			return fmt.Errorf("encode telegram review heads: %w", err)
		}

		heads = encoded
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, rememberQuery,
		message.BotID.String(), message.ChatID, message.MessageID, string(message.Kind),
		idOrEmpty(message.QuestionID), message.ExecutionID, message.PlanRevision, heads, time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("remember telegram decision message: %w", err)
	}

	return nil
}

func (r *telegramConversationRepository) DecisionAt(
	ctx context.Context,
	botID uuid.UUID,
	chatID, messageID int64,
) (entity.TelegramDecisionMessage, error) {
	message, err := scanDecision(r.db.Querier(ctx).QueryRowContext(
		ctx, decisionAtQuery, botID.String(), chatID, messageID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.TelegramDecisionMessage{}, entity.ErrTelegramDecisionNotFound
	}

	if err != nil {
		return entity.TelegramDecisionMessage{}, fmt.Errorf("find telegram decision message: %w", err)
	}

	return message, nil
}

func (r *telegramConversationRepository) Posted(
	ctx context.Context,
	decision entity.TelegramDecision,
) ([]entity.TelegramDecisionMessage, error) {
	query, args := postedForExecutionQuery, []any{string(decision.Kind), decision.ExecutionID}
	if decision.Kind == entity.TelegramDecisionQuestion {
		query, args = postedForQuestionQuery, []any{decision.QuestionID.String()}
	}

	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list telegram decision messages: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var posted []entity.TelegramDecisionMessage

	for rows.Next() {
		message, err := scanDecision(rows)
		if err != nil {
			return nil, fmt.Errorf("scan telegram decision message: %w", err)
		}

		posted = append(posted, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list telegram decision messages: %w", err)
	}

	return posted, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDecision(row scanner) (entity.TelegramDecisionMessage, error) {
	var (
		message  entity.TelegramDecisionMessage
		bot      string
		kind     string
		question string
		heads    []byte
	)

	if err := row.Scan(
		&bot, &message.ChatID, &message.MessageID, &kind, &question, &message.ExecutionID,
		&message.PlanRevision, &heads, &message.Settled,
	); err != nil {
		return entity.TelegramDecisionMessage{}, err
	}

	message.Kind = entity.TelegramDecisionKind(kind)

	var err error

	if message.BotID, err = uuid.Parse(bot); err != nil {
		return entity.TelegramDecisionMessage{}, fmt.Errorf("parse telegram decision bot id: %w", err)
	}

	if question != "" {
		if message.QuestionID, err = uuid.Parse(question); err != nil {
			return entity.TelegramDecisionMessage{}, fmt.Errorf("parse telegram decision question id: %w", err)
		}
	}

	if len(heads) > 0 {
		if err := json.Unmarshal(heads, &message.ReviewHeads); err != nil {
			return entity.TelegramDecisionMessage{}, fmt.Errorf("read telegram review heads: %w", err)
		}
	}

	return message, nil
}

func idOrEmpty(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func (r *telegramConversationRepository) MarkSettled(
	ctx context.Context,
	message entity.TelegramDecisionMessage,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, markSettledQuery, message.BotID.String(), message.ChatID, message.MessageID, at,
	); err != nil {
		return fmt.Errorf("settle telegram decision message: %w", err)
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

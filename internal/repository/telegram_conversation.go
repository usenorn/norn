package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=telegram_conversation.go -destination=telegramconversation/mock_telegram_conversation.go -package=telegramconversation -mock_names=TelegramConversation=MockTelegramConversation

type TelegramConversation interface {
	Remember(ctx context.Context, message entity.TelegramDecisionMessage) error
	DecisionAt(ctx context.Context, botID uuid.UUID, chatID, messageID int64) (entity.TelegramDecisionMessage, error)
	Posted(ctx context.Context, decision entity.TelegramDecision) ([]entity.TelegramDecisionMessage, error)
	MarkSettled(ctx context.Context, message entity.TelegramDecisionMessage, at time.Time) error
	Append(ctx context.Context, botID uuid.UUID, chatID int64, turn entity.AgentTurn) error
	History(ctx context.Context, botID uuid.UUID, chatID int64, limit int) ([]entity.AgentTurn, error)
	Prune(ctx context.Context, botID uuid.UUID, chatID int64, keep int) error
}

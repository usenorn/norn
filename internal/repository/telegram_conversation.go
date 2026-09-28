package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=telegram_conversation.go -destination=telegramconversation/mock_telegram_conversation.go -package=telegramconversation -mock_names=TelegramConversation=MockTelegramConversation

type TelegramConversation interface {
	Remember(ctx context.Context, message entity.TelegramQuestionMessage) error
	QuestionAt(ctx context.Context, botID uuid.UUID, chatID, messageID int64) (uuid.UUID, error)
	Posted(ctx context.Context, questionID uuid.UUID) ([]entity.TelegramQuestionMessage, error)
	MarkSettled(ctx context.Context, message entity.TelegramQuestionMessage, at time.Time) error
	Append(ctx context.Context, botID uuid.UUID, chatID int64, turn entity.AgentTurn) error
	History(ctx context.Context, botID uuid.UUID, chatID int64, limit int) ([]entity.AgentTurn, error)
	Prune(ctx context.Context, botID uuid.UUID, chatID int64, keep int) error
}

package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=telegram_bot.go -destination=telegrambot/mock_telegram_bot.go -package=telegrambot -mock_names=TelegramBot=MockTelegramBot

type TelegramBot interface {
	Get(ctx context.Context, workspaceID, agentID uuid.UUID) (entity.TelegramBot, error)
	GetByID(ctx context.Context, botID uuid.UUID) (entity.TelegramBot, error)
	GetByBotUser(ctx context.Context, botUserID int64) (entity.TelegramBot, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]entity.TelegramBot, error)
	Token(ctx context.Context, botID uuid.UUID) (string, error)
	SecretHash(ctx context.Context, botID uuid.UUID) ([]byte, error)
	Save(ctx context.Context, bot entity.TelegramBot, token string, secretHash []byte) (entity.TelegramBot, error)
	Rekey(ctx context.Context, bot entity.TelegramBot, token string, secretHash []byte) (entity.TelegramBot, error)
	Delete(ctx context.Context, workspaceID, agentID uuid.UUID) error
}

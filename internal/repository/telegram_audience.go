package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=telegram_audience.go -destination=telegramaudience/mock_telegram_audience.go -package=telegramaudience -mock_names=TelegramAudience=MockTelegramAudience

type TelegramAudience interface {
	IssueCode(ctx context.Context, code entity.TelegramLinkCode, hash []byte) error
	LinkCode(ctx context.Context, botID uuid.UUID, hash []byte, now time.Time) (entity.TelegramLinkCode, error)
	SpendCode(ctx context.Context, botID uuid.UUID, hash []byte) error
	Link(ctx context.Context, account entity.TelegramAccount) error
	AccountOf(ctx context.Context, botID uuid.UUID, telegramUserID int64) (entity.TelegramAccount, error)
	AccountFor(ctx context.Context, botID, accountID uuid.UUID) (entity.TelegramAccount, error)
	Accounts(ctx context.Context, botID uuid.UUID) ([]entity.TelegramAccount, error)
	Unlink(ctx context.Context, botID, accountID uuid.UUID) error
	Bind(ctx context.Context, group entity.TelegramGroup) (entity.TelegramGroup, error)
	Groups(ctx context.Context, botID uuid.UUID) ([]entity.TelegramGroup, error)
	Group(ctx context.Context, botID uuid.UUID, chatID int64) (entity.TelegramGroup, error)
	Unbind(ctx context.Context, botID, groupID uuid.UUID) (entity.TelegramGroup, error)
	UnbindChat(ctx context.Context, botID uuid.UUID, chatID int64) error
	MoveGroup(ctx context.Context, botID uuid.UUID, fromChatID, toChatID int64) error
}

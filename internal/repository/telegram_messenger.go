package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=telegram_messenger.go -destination=telegrammessenger/mock_telegram_messenger.go -package=telegrammessenger -mock_names=TelegramMessenger=MockTelegramMessenger

type TelegramMessenger interface {
	Identify(ctx context.Context, token string) (entity.TelegramIdentity, error)
	Register(ctx context.Context, token, url, secret string) error
	Unregister(ctx context.Context, token string) error
	Send(ctx context.Context, token string, message entity.TelegramOutgoing) (int64, error)
	Edit(ctx context.Context, token string, chatID, messageID int64, text string) error
	AnswerCallback(ctx context.Context, token, callbackID, text string) error
	Typing(ctx context.Context, token string, chatID int64) error
	Decode(payload []byte) (entity.TelegramIncoming, error)
}

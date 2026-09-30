package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=telegram_updates.go -destination=telegrambot/mock_telegram_updates.go -package=telegrambot -mock_names=TelegramUpdates=MockTelegramUpdates

type TelegramUpdates interface {
	Accept(ctx context.Context, botID uuid.UUID, secret string, payload []byte) error
	Apply(ctx context.Context, updateID uuid.UUID) error
	Relay(ctx context.Context, decision entity.TelegramDecision) error
	Settle(ctx context.Context, decision entity.TelegramDecision) error
	Sweep(ctx context.Context) (int, error)
}

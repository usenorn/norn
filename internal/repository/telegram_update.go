package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=telegram_update.go -destination=telegramupdate/mock_telegram_update.go -package=telegramupdate -mock_names=TelegramUpdate=MockTelegramUpdate

type TelegramUpdate interface {
	Record(ctx context.Context, update entity.TelegramUpdate) (uuid.UUID, error)
	UpdateOf(ctx context.Context, botID uuid.UUID, updateID int64) (entity.TelegramUpdate, error)
	Lock(ctx context.Context, id uuid.UUID) (entity.TelegramUpdate, error)
	Settle(ctx context.Context, id uuid.UUID, outcome entity.TelegramUpdateOutcome, at time.Time) error
	Sweep(ctx context.Context, before time.Time, limit int) (int, error)
}

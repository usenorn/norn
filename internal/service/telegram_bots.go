package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=telegram_bots.go -destination=telegrambot/mock_telegram_bots.go -package=telegrambot -mock_names=TelegramBots=MockTelegramBots

type TelegramBotView struct {
	Bot      entity.TelegramBot
	Hosted   bool
	Accounts []entity.TelegramAccount
	Groups   []entity.TelegramGroup
	Linked   bool
}

type TelegramLinkInvite struct {
	URL       string
	ExpiresAt time.Time
}

type MemberTelegramBot struct {
	Bot       entity.TelegramBot
	AgentName string
	Linked    bool
	LinkedAs  string
}

type TelegramBots interface {
	Get(ctx context.Context, workspaceID, agentID uuid.UUID) (TelegramBotView, error)
	Connect(ctx context.Context, workspaceID, agentID uuid.UUID, token string) (TelegramBotView, error)
	Disconnect(ctx context.Context, workspaceID, agentID uuid.UUID) error
	IssueLink(
		ctx context.Context,
		workspaceID, agentID uuid.UUID,
		purpose entity.TelegramLinkPurpose,
	) (TelegramLinkInvite, error)
	Unlink(ctx context.Context, workspaceID, agentID uuid.UUID) error
	Unbind(ctx context.Context, workspaceID, agentID, groupID uuid.UUID) error
	Mine(ctx context.Context, workspaceID uuid.UUID) ([]MemberTelegramBot, error)
}

package telegrambot_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func (h *harness) identifies(botUserID int64) {
	h.messenger.EXPECT().
		Identify(gomock.Any(), botToken).
		Return(entity.TelegramIdentity{BotUserID: botUserID, Username: "ada_bot", Name: "Ada"}, nil)
}

func (h *harness) emptyAudience() {
	h.audience.EXPECT().Accounts(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	h.audience.EXPECT().Groups(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
}

func TestConnectingRegistersTheWebhookWithTheSecretItStored(t *testing.T) {
	h := newHarness(t)
	h.emptyAudience()
	h.identifies(botUserID)

	h.bots.EXPECT().GetByBotUser(gomock.Any(), int64(botUserID)).Return(entity.TelegramBot{}, entity.ErrTelegramBotNotFound)
	h.bots.EXPECT().Get(gomock.Any(), h.workspaceID, h.agent.ID).Return(entity.TelegramBot{}, entity.ErrTelegramBotNotFound)

	var storedHash []byte

	h.bots.EXPECT().
		Save(gomock.Any(), gomock.Any(), botToken, gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entity.TelegramBot, _ string, hash []byte) (entity.TelegramBot, error) {
			if bot.BotUserID != botUserID || bot.AgentID != h.agent.ID || bot.ConnectedBy != h.caller.AccountID {
				t.Errorf("saved bot = %+v", bot)
			}

			storedHash = hash
			saved := bot
			saved.ID = h.bot.ID

			return saved, nil
		})

	h.messenger.EXPECT().
		Register(gomock.Any(), botToken, baseURL+"/v1/telegram/bots/"+h.bot.ID.String()+"/updates", gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, secret string) error {
			if !bytes.Equal(entity.HashTelegramSecret(secret), storedHash) {
				t.Error("the webhook secret does not match the stored hash")
			}

			return nil
		})

	view, err := h.botsService().Connect(context.Background(), h.workspaceID, h.agent.ID, "  "+botToken+"  ")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if view.Bot.ID != h.bot.ID || !view.Hosted {
		t.Errorf("view = %+v", view)
	}
}

func TestConnectingIsRefusedBeforeTelegramIsCalled(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(h *harness)
		token   string
		want    error
	}{
		{
			name:    "the instance is not served over https",
			arrange: func(h *harness) { h.app.BaseURL = "http://localhost:5174" },
			token:   botToken,
			want:    entity.ErrTelegramOriginInsecure,
		},
		{
			name:    "somebody who does not manage the agent",
			arrange: func(h *harness) { h.caller.AccountID = uuid.New() },
			token:   botToken,
			want:    entity.ErrAgentNotFound,
		},
		{
			name:    "an API token rather than a person",
			arrange: func(h *harness) { h.caller.Kind = entity.ActorKindToken },
			token:   botToken,
			want:    entity.ErrAccountForbidden,
		},
		{
			name:    "a disabled agent",
			arrange: func(h *harness) { h.agent.Status = entity.AgentStatusDisabled },
			token:   botToken,
			want:    entity.ErrAgentDisabled,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.arrange(h)

			_, err := h.botsService().Connect(context.Background(), h.workspaceID, h.agent.ID, tc.token)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAMalformedTokenIsAValidationError(t *testing.T) {
	h := newHarness(t)

	_, err := h.botsService().Connect(context.Background(), h.workspaceID, h.agent.ID, "not-a-token")

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "token" {
		t.Fatalf("err = %v, want a validation error on token", err)
	}
}

func TestABotAlreadyServingAnotherAgentIsRefused(t *testing.T) {
	h := newHarness(t)
	h.identifies(botUserID)
	h.bots.EXPECT().
		GetByBotUser(gomock.Any(), int64(botUserID)).
		Return(entity.TelegramBot{ID: uuid.New(), AgentID: uuid.New()}, nil)

	_, err := h.botsService().Connect(context.Background(), h.workspaceID, h.agent.ID, botToken)
	if !errors.Is(err, entity.ErrTelegramBotTaken) {
		t.Fatalf("err = %v, want ErrTelegramBotTaken", err)
	}
}

func TestReconnectingTheSameBotKeepsItsLinks(t *testing.T) {
	h := newHarness(t)
	h.emptyAudience()
	h.identifies(botUserID)

	h.bots.EXPECT().GetByBotUser(gomock.Any(), int64(botUserID)).Return(h.bot, nil)
	h.bots.EXPECT().Get(gomock.Any(), h.workspaceID, h.agent.ID).Return(h.bot, nil)
	h.bots.EXPECT().
		Rekey(gomock.Any(), gomock.Any(), botToken, gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entity.TelegramBot, _ string, _ []byte) (entity.TelegramBot, error) {
			if bot.ID != h.bot.ID {
				t.Errorf("rekeyed bot %s, want %s", bot.ID, h.bot.ID)
			}

			return bot, nil
		})
	h.messenger.EXPECT().Register(gomock.Any(), botToken, gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.botsService().Connect(context.Background(), h.workspaceID, h.agent.ID, botToken); err != nil {
		t.Fatalf("Connect: %v", err)
	}
}

func TestAWebhookTelegramRefusesRollsTheConnectionBack(t *testing.T) {
	h := newHarness(t)
	h.identifies(botUserID)

	h.bots.EXPECT().GetByBotUser(gomock.Any(), int64(botUserID)).Return(entity.TelegramBot{}, entity.ErrTelegramBotNotFound)
	h.bots.EXPECT().Get(gomock.Any(), h.workspaceID, h.agent.ID).Return(entity.TelegramBot{}, entity.ErrTelegramBotNotFound)
	h.bots.EXPECT().Save(gomock.Any(), gomock.Any(), botToken, gomock.Any()).Return(h.bot, nil)
	h.messenger.EXPECT().Register(gomock.Any(), botToken, gomock.Any(), gomock.Any()).Return(entity.ErrTelegramUnreachable)

	_, err := h.botsService().Connect(context.Background(), h.workspaceID, h.agent.ID, botToken)
	if !errors.Is(err, entity.ErrTelegramUnreachable) {
		t.Fatalf("err = %v, want ErrTelegramUnreachable so the transaction rolls back", err)
	}
}

func TestOnlyAManagerMayIssueAGroupLink(t *testing.T) {
	h := newHarness(t)
	h.caller.AccountID = uuid.New()

	_, err := h.botsService().IssueLink(context.Background(), h.workspaceID, h.agent.ID, entity.TelegramLinkGroup)
	if !errors.Is(err, entity.ErrAgentNotFound) {
		t.Fatalf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestAMemberGetsAPrivateLinkBoundToThemselves(t *testing.T) {
	h := newHarness(t)
	h.caller.AccountID = uuid.New()
	h.bots.EXPECT().Get(gomock.Any(), h.workspaceID, h.agent.ID).Return(h.bot, nil)

	var issued entity.TelegramLinkCode

	var hash []byte

	h.audience.EXPECT().
		IssueCode(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, code entity.TelegramLinkCode, codeHash []byte) error {
			issued, hash = code, codeHash

			return nil
		})

	invite, err := h.botsService().IssueLink(context.Background(), h.workspaceID, h.agent.ID, entity.TelegramLinkPrivate)
	if err != nil {
		t.Fatalf("IssueLink: %v", err)
	}

	if issued.AccountID != h.caller.AccountID || issued.BotID != h.bot.ID || issued.Purpose != entity.TelegramLinkPrivate {
		t.Errorf("issued = %+v", issued)
	}

	code, found := strings.CutPrefix(invite.URL, "https://t.me/ada_bot?start=")
	if !found {
		t.Fatalf("url = %q", invite.URL)
	}

	if !bytes.Equal(entity.HashTelegramSecret(code), hash) {
		t.Error("the link carries a code whose hash was not stored")
	}
}

package telegrambot_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	messengerrepo "github.com/usenorn/norn/internal/repository/telegrammessenger"
)

func (h *harness) relaying(asked entity.IssueQuestion) {
	h.questions.EXPECT().GetByID(gomock.Any(), h.workspaceID, asked.ID).Return(asked, nil)
	h.bots.EXPECT().Get(gomock.Any(), h.workspaceID, h.agent.ID).Return(h.bot, nil)
	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, asked.IssueID, gomock.Any()).
		Return(entity.Issue{ID: asked.IssueID, ReferenceKey: "NORN", Number: 7, Title: "Ship <the> release"}, nil).
		AnyTimes()
}

func TestAQuestionGoesToTheDelegatorAndEveryBoundGroupOnce(t *testing.T) {
	h := newHarness(t)
	asked := h.question("Friday", "Monday")
	delegator := uuid.New()
	h.relaying(asked)

	h.delegations.EXPECT().
		Open(gomock.Any(), h.workspaceID, asked.IssueID).
		Return(entity.IssueDelegation{DelegatedByAccountID: delegator}, nil)
	h.audience.EXPECT().AccountFor(gomock.Any(), h.bot.ID, delegator).Return(h.linked(delegator, 900), nil)
	h.audience.EXPECT().Groups(gomock.Any(), h.bot.ID).Return([]entity.TelegramGroup{{ChatID: groupChat}, {ChatID: -55}}, nil)
	h.conversation.EXPECT().
		Posted(gomock.Any(), asked.ID).
		Return([]entity.TelegramQuestionMessage{{BotID: h.bot.ID, ChatID: -55, MessageID: 3, QuestionID: asked.ID}}, nil)

	var remembered []int64

	h.conversation.EXPECT().
		Remember(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, message entity.TelegramQuestionMessage) error {
			if message.QuestionID != asked.ID || message.MessageID == 0 {
				t.Errorf("remembered %+v", message)
			}

			remembered = append(remembered, message.ChatID)

			return nil
		}).
		Times(2)

	if err := h.updatesService().Relay(context.Background(), h.workspaceID, asked.ID); err != nil {
		t.Fatalf("Relay: %v", err)
	}

	if !slices.Equal(remembered, []int64{900, groupChat}) {
		t.Errorf("sent to %v, want the delegator then the unposted group", remembered)
	}

	message := h.sent[0]
	if !slices.Equal(message.Options, asked.Options) {
		t.Errorf("options = %q", message.Options)
	}

	for _, want := range []string{
		`<a href="https://norn.example/acme/issues/NORN-7">NORN-7</a>`,
		"Ship &lt;the&gt; release",
		"Ship it on Friday?",
		"30 Sep 2026, 12:00 UTC",
		"<i>Monday</i>",
	} {
		if !strings.Contains(message.Text, want) {
			t.Errorf("message lacks %q:\n%s", want, message.Text)
		}
	}
}

func TestAnAgentWithoutABotRelaysNothing(t *testing.T) {
	h := newHarness(t)
	asked := h.question()
	h.questions.EXPECT().GetByID(gomock.Any(), h.workspaceID, asked.ID).Return(asked, nil)
	h.bots.EXPECT().Get(gomock.Any(), h.workspaceID, h.agent.ID).Return(entity.TelegramBot{}, entity.ErrTelegramBotNotFound)

	if err := h.updatesService().Relay(context.Background(), h.workspaceID, asked.ID); err != nil {
		t.Fatalf("Relay: %v", err)
	}
}

func TestAQuestionAskedByAPersonRelaysNothing(t *testing.T) {
	h := newHarness(t)
	asked := h.question()
	asked.AskedByAccountID = uuid.New()
	h.questions.EXPECT().GetByID(gomock.Any(), h.workspaceID, asked.ID).Return(asked, nil)
	h.agents.EXPECT().GetByAccountID(gomock.Any(), asked.AskedByAccountID).Return(entity.Agent{}, entity.ErrAgentNotFound)

	if err := h.updatesService().Relay(context.Background(), h.workspaceID, asked.ID); err != nil {
		t.Fatalf("Relay: %v", err)
	}
}

func TestASettledQuestionIsEditedToItsOutcomeEverywhereItWasPosted(t *testing.T) {
	h := newHarness(t)
	asked := h.question("Friday", "Monday")
	answeredAt := time.Now()
	asked.State = entity.QuestionAnswered
	asked.Answer = "Friday"
	asked.AnsweredByName = "Rae"
	asked.AnsweredAt = &answeredAt
	h.relaying(asked)

	posted := []entity.TelegramQuestionMessage{
		{BotID: h.bot.ID, ChatID: 900, MessageID: 1, QuestionID: asked.ID},
		{BotID: h.bot.ID, ChatID: groupChat, MessageID: 2, QuestionID: asked.ID, Settled: true},
	}
	h.conversation.EXPECT().Posted(gomock.Any(), asked.ID).Return(posted, nil)
	h.messenger.EXPECT().
		Edit(gomock.Any(), botToken, int64(900), int64(1), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _, _ int64, text string) error {
			if !strings.Contains(text, "Answered by Rae: <i>Friday</i>") {
				t.Errorf("edited to:\n%s", text)
			}

			return nil
		})
	h.conversation.EXPECT().MarkSettled(gomock.Any(), posted[0], gomock.Any()).Return(nil)

	if err := h.updatesService().Settle(context.Background(), h.workspaceID, asked.ID); err != nil {
		t.Fatalf("Settle: %v", err)
	}
}

func TestABlockedChatDoesNotStopTheOthers(t *testing.T) {
	h := newHarness(t)
	asked := h.question()
	h.relaying(asked)

	h.delegations.EXPECT().Open(gomock.Any(), h.workspaceID, asked.IssueID).Return(entity.IssueDelegation{}, entity.ErrIssueDelegationNotFound)
	h.audience.EXPECT().Groups(gomock.Any(), h.bot.ID).Return([]entity.TelegramGroup{{ChatID: -1}, {ChatID: -2}}, nil)
	h.conversation.EXPECT().Posted(gomock.Any(), asked.ID).Return(nil, nil)

	h.messenger = messengerrepo.NewMockTelegramMessenger(gomock.NewController(t))
	h.messenger.EXPECT().Send(gomock.Any(), botToken, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, message entity.TelegramOutgoing) (int64, error) {
			if message.ChatID == -1 {
				return 0, entity.ErrTelegramChatUnavailable
			}

			return 5, nil
		}).
		Times(2)
	h.conversation.EXPECT().
		Remember(gomock.Any(), entity.TelegramQuestionMessage{BotID: h.bot.ID, ChatID: -2, MessageID: 5, QuestionID: asked.ID}).
		Return(nil)

	if err := h.updatesService().Relay(context.Background(), h.workspaceID, asked.ID); err != nil {
		t.Fatalf("Relay: %v", err)
	}
}

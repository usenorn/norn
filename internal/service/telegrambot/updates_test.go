package telegrambot_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

const secret = "webhook-secret"

func (h *harness) accepting() {
	h.bots.EXPECT().SecretHash(gomock.Any(), h.bot.ID).Return(entity.HashTelegramSecret(secret), nil)
}

func groupChatter(text string) entity.TelegramIncoming {
	return entity.TelegramIncoming{UpdateID: 1, Message: &entity.TelegramMessage{
		ChatID:    groupChat,
		ChatType:  entity.TelegramChatSupergroup,
		MessageID: 10,
		Sender:    entity.TelegramSender{ID: senderID},
		Text:      text,
	}}
}

func privately(text string) entity.TelegramIncoming {
	return entity.TelegramIncoming{UpdateID: 2, Message: &entity.TelegramMessage{
		ChatID:    senderID,
		ChatType:  entity.TelegramChatPrivate,
		MessageID: privateMsg,
		Sender:    entity.TelegramSender{ID: senderID, Username: "rae"},
		Text:      text,
	}}
}

func TestAnUpdateWithTheWrongSecretIsRefused(t *testing.T) {
	h := newHarness(t)
	h.accepting()

	err := h.updatesService().Accept(context.Background(), h.bot.ID, "guessed", []byte(`{}`))
	if !errors.Is(err, entity.ErrTelegramSecretInvalid) {
		t.Fatalf("err = %v, want ErrTelegramSecretInvalid", err)
	}
}

func TestGroupChatterNotAddressedToTheBotIsNeverStored(t *testing.T) {
	h := newHarness(t)
	h.accepting()
	h.messenger.EXPECT().Decode(gomock.Any()).Return(groupChatter("lunch?"), nil)

	if err := h.updatesService().Accept(context.Background(), h.bot.ID, secret, []byte(`{}`)); err != nil {
		t.Fatalf("Accept: %v", err)
	}
}

func TestARedeliveredUpdateIsQueuedAgainOnlyIfItWasNeverApplied(t *testing.T) {
	for _, processed := range []bool{false, true} {
		h := newHarness(t)
		h.accepting()
		h.messenger.EXPECT().Decode(gomock.Any()).Return(privately("hello"), nil)
		h.received.EXPECT().Record(gomock.Any(), gomock.Any()).Return(uuid.Nil, entity.ErrTelegramUpdateDuplicate)

		stored := entity.TelegramUpdate{ID: uuid.New(), BotID: h.bot.ID, UpdateID: 2}
		if processed {
			at := time.Now()
			stored.ProcessedAt = &at
		} else {
			h.jobs.EXPECT().EnqueueTelegramUpdate(gomock.Any(), entity.TelegramUpdatePayload{UpdateID: stored.ID}).Return(nil)
		}

		h.received.EXPECT().UpdateOf(gomock.Any(), h.bot.ID, int64(2)).Return(stored, nil)

		if err := h.updatesService().Accept(context.Background(), h.bot.ID, secret, []byte(`{}`)); err != nil {
			t.Fatalf("Accept (processed=%v): %v", processed, err)
		}
	}
}

func TestStartWithAPrivateCodeLinksTheTelegramUserToItsAccount(t *testing.T) {
	h := newHarness(t)
	code, hash, err := entity.NewTelegramLinkCode()
	if err != nil {
		t.Fatal(err)
	}

	accountID := uuid.New()
	id := h.applying(privately("/start " + code))
	h.audience.EXPECT().LinkCode(gomock.Any(), h.bot.ID, hash, gomock.Any()).Return(entity.TelegramLinkCode{
		BotID: h.bot.ID, AccountID: accountID, Purpose: entity.TelegramLinkPrivate,
	}, nil)
	h.audience.EXPECT().Link(gomock.Any(), entity.TelegramAccount{
		BotID: h.bot.ID, AccountID: accountID, TelegramUserID: senderID, ChatID: senderID, Username: "rae",
	}).Return(nil)
	h.audience.EXPECT().SpendCode(gomock.Any(), h.bot.ID, hash).Return(nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(h.sent) != 1 || !strings.HasPrefix(h.sent[0].Text, "Linked.") {
		t.Errorf("sent = %+v", h.sent)
	}
}

func TestAGroupRefusedToAnotherPersonKeepsItsLinkForTheCreator(t *testing.T) {
	for name, linked := range map[string]bool{"unlinked sender": false, "another linked account": true} {
		h := newHarness(t)
		code, hash, err := entity.NewTelegramLinkCode()
		if err != nil {
			t.Fatal(err)
		}

		id := h.applying(groupChatter("/start@ada_bot " + code))
		h.audience.EXPECT().LinkCode(gomock.Any(), h.bot.ID, hash, gomock.Any()).Return(entity.TelegramLinkCode{
			BotID: h.bot.ID, AccountID: uuid.New(), Purpose: entity.TelegramLinkGroup,
		}, nil)

		if linked {
			h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(uuid.New(), senderID), nil)
		} else {
			h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).
				Return(entity.TelegramAccount{}, entity.ErrTelegramAccountNotLinked)
		}

		h.settles(id, entity.TelegramUpdateApplied)

		if err := h.updatesService().Apply(context.Background(), id); err != nil {
			t.Fatalf("%s: Apply: %v", name, err)
		}

		if len(h.sent) != 1 || !strings.HasPrefix(h.sent[0].Text, "Only the person who created this link") {
			t.Errorf("%s: sent = %+v", name, h.sent)
		}
	}
}

func TestTheCreatorOfAGroupLinkConnectsTheGroupAndSpendsTheLink(t *testing.T) {
	h := newHarness(t)
	code, hash, err := entity.NewTelegramLinkCode()
	if err != nil {
		t.Fatal(err)
	}

	accountID := uuid.New()
	incoming := groupChatter("/start@ada_bot " + code)
	incoming.Message.ChatTitle = "Launch"
	id := h.applying(incoming)
	h.audience.EXPECT().LinkCode(gomock.Any(), h.bot.ID, hash, gomock.Any()).Return(entity.TelegramLinkCode{
		BotID: h.bot.ID, AccountID: accountID, Purpose: entity.TelegramLinkGroup,
	}, nil)
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(accountID, senderID), nil)
	bound := entity.TelegramGroup{BotID: h.bot.ID, ChatID: groupChat, Title: "Launch", BoundBy: accountID}
	h.audience.EXPECT().Bind(gomock.Any(), bound).Return(bound, nil)
	h.audience.EXPECT().SpendCode(gomock.Any(), h.bot.ID, hash).Return(nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(h.sent) != 1 || !strings.HasPrefix(h.sent[0].Text, "Connected.") {
		t.Errorf("sent = %+v", h.sent)
	}
}

func TestALinkOpenedInTheWrongKindOfChatIsExplainedAndKept(t *testing.T) {
	cases := []struct {
		name     string
		purpose  entity.TelegramLinkPurpose
		incoming func(string) entity.TelegramIncoming
		reply    string
	}{
		{"group link in a private chat", entity.TelegramLinkGroup, privately, "This link connects a group."},
		{"private link in a group", entity.TelegramLinkPrivate, groupChatter, "This link is for your private chat"},
	}

	for _, c := range cases {
		h := newHarness(t)
		code, hash, err := entity.NewTelegramLinkCode()
		if err != nil {
			t.Fatal(err)
		}

		id := h.applying(c.incoming("/start " + code))
		h.audience.EXPECT().LinkCode(gomock.Any(), h.bot.ID, hash, gomock.Any()).Return(entity.TelegramLinkCode{
			BotID: h.bot.ID, AccountID: uuid.New(), Purpose: c.purpose,
		}, nil)
		h.settles(id, entity.TelegramUpdateApplied)

		if err := h.updatesService().Apply(context.Background(), id); err != nil {
			t.Fatalf("%s: Apply: %v", c.name, err)
		}

		if len(h.sent) != 1 || !strings.HasPrefix(h.sent[0].Text, c.reply) {
			t.Errorf("%s: sent = %+v", c.name, h.sent)
		}
	}
}

func TestAnExpiredOrUsedLinkSaysSo(t *testing.T) {
	h := newHarness(t)
	code, hash, err := entity.NewTelegramLinkCode()
	if err != nil {
		t.Fatal(err)
	}

	id := h.applying(groupChatter("/start@ada_bot " + code))
	h.audience.EXPECT().LinkCode(gomock.Any(), h.bot.ID, hash, gomock.Any()).
		Return(entity.TelegramLinkCode{}, entity.ErrTelegramLinkCodeInvalid)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(h.sent) != 1 || !strings.HasPrefix(h.sent[0].Text, "This link has expired or was already used.") {
		t.Errorf("sent = %+v", h.sent)
	}
}

func (h *harness) question(options ...string) entity.IssueQuestion {
	return entity.IssueQuestion{
		ID:               uuid.New(),
		WorkspaceID:      h.workspaceID,
		IssueID:          uuid.New(),
		Question:         "Ship it on Friday?",
		DefaultAnswer:    "Monday",
		Options:          options,
		AllowFreeText:    true,
		State:            entity.QuestionAsked,
		Deadline:         time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
		AskedByAccountID: h.agent.AccountID,
	}
}

func (h *harness) posted(asked entity.IssueQuestion) entity.TelegramDecisionMessage {
	return entity.TelegramDecisionMessage{
		BotID: h.bot.ID, ChatID: groupChat, MessageID: 77, Kind: entity.TelegramDecisionQuestion, QuestionID: asked.ID,
	}
}

func TestATappedOptionAnswersAsTheLinkedAccount(t *testing.T) {
	h := newHarness(t)
	asked := h.question("Friday", "Monday")
	accountID := uuid.New()

	id := h.applying(entity.TelegramIncoming{UpdateID: 3, Callback: &entity.TelegramCallback{
		ID: "cb-1", Sender: entity.TelegramSender{ID: senderID}, ChatID: groupChat, MessageID: 77, Data: "a:1",
	}})
	h.conversation.EXPECT().DecisionAt(gomock.Any(), h.bot.ID, int64(groupChat), int64(77)).Return(h.posted(asked), nil)
	h.questions.EXPECT().GetByID(gomock.Any(), h.workspaceID, asked.ID).Return(asked, nil)
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(accountID, senderID), nil)
	h.answers.EXPECT().
		Answer(gomock.Any(), h.workspaceID, asked.IssueID, asked.ID, service.AnswerQuestionInput{Answer: "Monday"}).
		DoAndReturn(func(ctx context.Context, _, _, _ uuid.UUID, _ service.AnswerQuestionInput) (entity.IssueQuestion, error) {
			actor, _ := identity.Actor(ctx)
			if actor.AccountID != accountID || actor.ConnectionName != entity.TelegramConnectionName {
				t.Errorf("answered as %+v, want the linked account through Telegram", actor)
			}

			return asked, nil
		})
	h.messenger.EXPECT().AnswerCallback(gomock.Any(), botToken, "cb-1", "Answered.").Return(nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestAnUnlinkedTapIsToldToLinkAndAnswersNothing(t *testing.T) {
	h := newHarness(t)
	asked := h.question("Friday", "Monday")

	id := h.applying(entity.TelegramIncoming{UpdateID: 4, Callback: &entity.TelegramCallback{
		ID: "cb-2", Sender: entity.TelegramSender{ID: senderID}, ChatID: groupChat, MessageID: 77, Data: "a:0",
	}})
	h.conversation.EXPECT().DecisionAt(gomock.Any(), h.bot.ID, int64(groupChat), int64(77)).Return(h.posted(asked), nil)
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(entity.TelegramAccount{}, entity.ErrTelegramAccountNotLinked)
	h.messenger.EXPECT().AnswerCallback(gomock.Any(), botToken, "cb-2", "Link your Telegram in Norn to decide.").Return(nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestAForgedOptionIndexIsRefused(t *testing.T) {
	h := newHarness(t)
	asked := h.question("Friday")

	id := h.applying(entity.TelegramIncoming{UpdateID: 5, Callback: &entity.TelegramCallback{
		ID: "cb-3", Sender: entity.TelegramSender{ID: senderID}, ChatID: groupChat, MessageID: 77, Data: "a:9",
	}})
	h.conversation.EXPECT().DecisionAt(gomock.Any(), h.bot.ID, int64(groupChat), int64(77)).Return(h.posted(asked), nil)
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(uuid.New(), senderID), nil)
	h.questions.EXPECT().GetByID(gomock.Any(), h.workspaceID, asked.ID).Return(asked, nil)
	h.messenger.EXPECT().AnswerCallback(gomock.Any(), botToken, "cb-3", "That option is not available.").Return(nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestAReplyToAQuestionAnswersItInFreeText(t *testing.T) {
	h := newHarness(t)
	asked := h.question()
	accountID := uuid.New()

	incoming := groupChatter("Wednesday works")
	incoming.Message.ReplyToID = 77
	incoming.Message.ReplyToSenderID = botUserID
	id := h.applying(incoming)

	h.audience.EXPECT().Group(gomock.Any(), h.bot.ID, int64(groupChat)).Return(entity.TelegramGroup{}, nil)
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(accountID, senderID), nil)
	h.conversation.EXPECT().DecisionAt(gomock.Any(), h.bot.ID, int64(groupChat), int64(77)).Return(h.posted(asked), nil)
	h.questions.EXPECT().GetByID(gomock.Any(), h.workspaceID, asked.ID).Return(asked, nil)
	h.answers.EXPECT().
		Answer(gomock.Any(), h.workspaceID, asked.IssueID, asked.ID, service.AnswerQuestionInput{Answer: "Wednesday works"}).
		Return(asked, nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(h.sent) != 1 || h.sent[0].Text != "Answered." || h.sent[0].ReplyTo != 10 {
		t.Errorf("sent = %+v", h.sent)
	}
}

func TestARunnerAgentsBotExplainsItOnlyAsksQuestions(t *testing.T) {
	h := newHarness(t)
	h.agent.Execution = entity.AgentExecutionRunner

	id := h.applying(privately("what are you working on?"))
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(uuid.New(), senderID), nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(h.sent) != 1 || !strings.Contains(h.sent[0].Text, "works through its runner") {
		t.Errorf("sent = %+v", h.sent)
	}
}

func TestAHostedAgentAnswersWithItsRecentHistoryAfterTheUpdateIsSettled(t *testing.T) {
	h := newHarness(t)
	account := h.linked(h.agent.OwnerAccountID, senderID)
	history := []entity.AgentTurn{
		{Role: entity.AgentTurnAssistant, Text: "an orphaned reply"},
		{Role: entity.AgentTurnUser, Text: "hi"},
		{Role: entity.AgentTurnAssistant, Text: "hello"},
		{Role: entity.AgentTurnUser, Text: "what is <blocked>?"},
	}

	id := h.applying(privately("what is <blocked>?"))
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(account, nil)

	settled := false
	h.received.EXPECT().Settle(gomock.Any(), id, entity.TelegramUpdateApplied, gomock.Any()).
		DoAndReturn(func(context.Context, uuid.UUID, entity.TelegramUpdateOutcome, time.Time) error {
			settled = true

			return nil
		})

	h.messenger.EXPECT().Typing(gomock.Any(), botToken, int64(senderID)).Return(nil)
	h.conversation.EXPECT().
		Append(gomock.Any(), h.bot.ID, int64(senderID), entity.AgentTurn{Role: entity.AgentTurnUser, Text: "what is <blocked>?"}).
		DoAndReturn(func(context.Context, uuid.UUID, int64, entity.AgentTurn) error {
			if !settled {
				t.Error("the conversation started before the update was settled, so a retry would answer twice")
			}

			return nil
		})
	h.conversation.EXPECT().History(gomock.Any(), h.bot.ID, int64(senderID), 20).Return(history, nil)
	h.hosted.EXPECT().
		Chat(gomock.Any(), h.workspaceID, h.agent.ID, history[1:]).
		Return(entity.AgentReply{Text: "NORN-7 waits on <review> & tests."}, nil)
	h.conversation.EXPECT().
		Append(gomock.Any(), h.bot.ID, int64(senderID), entity.AgentTurn{Role: entity.AgentTurnAssistant, Text: "NORN-7 waits on <review> & tests."}).
		Return(nil)
	h.conversation.EXPECT().Prune(gomock.Any(), h.bot.ID, int64(senderID), 20).Return(nil)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(h.sent) != 1 || h.sent[0].Text != "NORN-7 waits on &lt;review&gt; &amp; tests." {
		t.Errorf("sent = %+v", h.sent)
	}
}

func TestSomebodyWhoCannotManageTheAgentIsToldWhoCan(t *testing.T) {
	h := newHarness(t)

	id := h.applying(privately("status?"))
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(uuid.New(), senderID), nil)
	h.settles(id, entity.TelegramUpdateApplied)
	h.messenger.EXPECT().Typing(gomock.Any(), botToken, gomock.Any()).Return(nil)
	h.conversation.EXPECT().Append(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.conversation.EXPECT().History(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entity.AgentTurn{{Role: entity.AgentTurnUser, Text: "status?"}}, nil)
	h.hosted.EXPECT().Chat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(entity.AgentReply{}, entity.ErrAgentNotFound)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(h.sent) != 1 || !strings.HasPrefix(h.sent[0].Text, "Only Ada&#39;s owner or a workspace admin") {
		t.Errorf("sent = %+v", h.sent)
	}
}

func TestAnAgentThatStopsWithoutAnAnswerSaysWhy(t *testing.T) {
	cases := map[entity.AgentConversationStop]string{
		entity.AgentConversationAnswered:   "Ada had nothing to say.",
		entity.AgentConversationRoundLimit: "Ada stopped after using as many tools as one answer may.",
		entity.AgentConversationTokenLimit: "Ada stopped at this instance&#39;s limit on model usage for one answer.",
		entity.AgentConversationTimeLimit:  "Ada ran out of time before it finished.",
	}

	for stop, want := range cases {
		h := newHarness(t)

		id := h.applying(privately("say hi"))
		h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).
			Return(h.linked(h.agent.OwnerAccountID, senderID), nil)
		h.settles(id, entity.TelegramUpdateApplied)
		h.messenger.EXPECT().Typing(gomock.Any(), botToken, gomock.Any()).Return(nil)
		h.conversation.EXPECT().Append(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		h.conversation.EXPECT().History(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entity.AgentTurn{{Role: entity.AgentTurnUser, Text: "say hi"}}, nil)
		h.hosted.EXPECT().Chat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entity.AgentReply{Stop: stop}, nil)

		if err := h.updatesService().Apply(context.Background(), id); err != nil {
			t.Fatalf("%s: Apply: %v", stop, err)
		}

		if len(h.sent) != 1 || h.sent[0].Text != want {
			t.Errorf("%s: sent = %+v, want %q", stop, h.sent, want)
		}
	}
}

func TestAnAlreadyAppliedUpdateIsNotAppliedAgain(t *testing.T) {
	h := newHarness(t)
	id := uuid.New()
	at := time.Now()
	h.received.EXPECT().Lock(gomock.Any(), id).Return(entity.TelegramUpdate{ID: id, BotID: h.bot.ID, ProcessedAt: &at}, nil)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestBeingRemovedFromAGroupUnbindsIt(t *testing.T) {
	h := newHarness(t)
	id := h.applying(entity.TelegramIncoming{UpdateID: 6, Membership: &entity.TelegramMembership{
		ChatID: groupChat, ChatType: entity.TelegramChatSupergroup, Removed: true,
	}})
	h.audience.EXPECT().UnbindChat(gomock.Any(), h.bot.ID, int64(groupChat)).Return(nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}
